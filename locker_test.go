package lockerd

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

var client *redis.Client = redis.NewClient(&redis.Options{
	Network:  "tcp",
	Addr:     "127.0.0.1:6379",
	Password: "",
	DB:       0,
})

func TestLock(t *testing.T) {
	l := New(client, LockerOptions{
		Key: "k1",
	})
	ctx := context.Background()

	t.Run("lock and unlock", func(t *testing.T) {
		unlock, err := l.Lock(ctx)
		if err != nil {
			t.Error("lock error")
		}

		err = unlock(ctx)
		if err != nil {
			t.Error("unlock error")
		}
	})

	t.Run("sleep to TTL", func(t *testing.T) {
		unlock, err := l.Lock(ctx)
		if err != nil {
			t.Error("lock error")
		}

		<-time.After(6 * time.Second)

		err = unlock(ctx)
		if err != ErrLockKeyIsNotSet {
			t.Error("lock should be expired")
		}
	})

	t.Run("context DeadlineExceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Nanosecond)
		defer cancel()

		unlock, err := l.Lock(ctx)
		if err != context.DeadlineExceeded {
			t.Error("error should be context.DeadlineExceeded")
		}

		if unlock != nil {
			t.Error("unlock should nil")
		}
	})

	t.Run("concurrency", func(t *testing.T) {
		wg := sync.WaitGroup{}

		for range 1000 {
			wg.Add(1)
			go func() {
				defer wg.Done()

				unlock, err := l.Lock(ctx)
				if err != nil {
					t.Error("Lock error")
				} else {
					if l.GetValue() == "" {
						t.Error("value shouldn't be empty before unlock")
					}

					err := unlock(ctx)
					if err != nil {
						t.Error("Unlock error")
					}
				}
			}()
		}

		wg.Wait()
	})
}

func TestTryLock(t *testing.T) {
	l := New(client, LockerOptions{
		Key: "k1",
	})
	ctx := context.Background()

	t.Run("lock and unlock", func(t *testing.T) {
		unlock, err := l.TryLock(ctx)
		if err != nil {
			t.Error("lock err")
		}

		err = unlock(ctx)
		if err != nil {
			t.Error("unlock error")
		}
	})

	t.Run("sleep to TTL", func(t *testing.T) {
		unlock, err := l.TryLock(ctx)
		if err != nil {
			t.Error("lock error")
		}

		<-time.After(6 * time.Second)

		err = unlock(ctx)
		if err != ErrLockKeyIsNotSet {
			t.Error("lock should be expired")
		}
	})

	t.Run("context DeadlineExceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Nanosecond)
		defer cancel()

		unlock, err := l.TryLock(ctx)
		if err != context.DeadlineExceeded {
			t.Error("error should be context.DeadlineExceeded")
		}

		if unlock != nil {
			t.Error("unlock should nil")
		}
	})

	t.Run("TryLock fail", func(t *testing.T) {
		unlock, err := l.TryLock(ctx)
		if err != nil {
			t.Error("lock error")
		}
		defer unlock(ctx)

		unlock, err = l.TryLock(ctx)
		if err != ErrLockIsAcquired {
			t.Error("error should be ErrLockIsAcquired")
		}

		if unlock != nil {
			t.Error("unlock should be nil")
		}
	})
}

func TestLockWithAutoRenewal(t *testing.T) {
	l := New(client, LockerOptions{
		Key: "k1",
	})
	ctx := context.Background()

	t.Run("lock and unlock", func(t *testing.T) {
		unlock, errChan, err := l.LockWithAutoRenewal(ctx)
		if err != nil {
			t.Error("lock error")
		}
		if errChan == nil {
			t.Error("errChan shouldn't be nil")
		}
		if len(errChan) != 0 {
			t.Error("errChan len is not 0")
		}
		if cap(errChan) != 1 {
			t.Error("errChan cap is not 1")
		}

		err = unlock(ctx)
		if err != nil {
			t.Error("unlock error")
		}

		_, ok := <-errChan
		if ok {
			t.Error("errChan should be closed and no value inside")
		}
	})

	t.Run("context DeadlineExceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Nanosecond)
		defer cancel()

		unlock, errChan, err := l.LockWithAutoRenewal(ctx)
		if err != context.DeadlineExceeded {
			t.Error("error should be context.DeadlineExceeded")
		}
		if unlock != nil {
			t.Error("unlock should be nil")
		}
		if errChan != nil {
			t.Error("errChan should be nil")
		}
	})

	t.Run("sleep to TTl", func(t *testing.T) {
		unlock, errChan, err := l.LockWithAutoRenewal(ctx)
		if err != nil {
			t.Error("lock error")
		}
		defer unlock(ctx)

		go func() {
			for range errChan {
				t.Error("errChan send a error")
			}
		}()

		if unlock == nil {
			t.Error("unlock is nil")
		}

		<-time.After(6 * time.Second)

		unlock, errChan, err = l.TryLockWithAutoRenewal(ctx)
		if err != ErrLockIsAcquired {
			t.Error("error should be ErrLockIsAcquired")
		}

		if errChan != nil {
			t.Error("errChan should be nil")
		}

		if unlock != nil {
			t.Error("unlock should be nil")
		}
	})

	t.Run("concurrency", func(t *testing.T) {
		wg := sync.WaitGroup{}

		for range 500 {
			wg.Add(1)
			go func() {
				unlock, errChan, err := l.LockWithAutoRenewal(ctx)
				if err != nil {
					t.Error("lock error")
				} else {
					if l.GetValue() == "" {
						t.Error("value shouldn't be empty before unlock")
					}

					err := unlock(ctx)
					if err != nil {
						t.Error("unlcok error")
					}

					for range errChan {
						t.Error("errChan send a error")
					}
				}

				wg.Done()
			}()
		}

		wg.Wait()
	})

	t.Run("concurrency 2", func(t *testing.T) {
		wg := sync.WaitGroup{}

		for range 200 {
			wg.Add(1)
			go func() {
				unlock, _, err := l.LockWithAutoRenewal(ctx)
				if err != nil {
					t.Error("lock error")
				} else {
					err := unlock(ctx)
					if err != nil {
						t.Error("unlock error")
					}
				}

				wg.Done()
			}()
		}

		wg.Wait()
	})
}

func TestTryLockWithAutoRenewal(t *testing.T) {
	l := New(client, LockerOptions{
		Key: "k1",
	})
	ctx := context.Background()

	t.Run("lock and unlock", func(t *testing.T) {
		unlock, errChan, err := l.TryLockWithAutoRenewal(ctx)
		if err != nil {
			t.Error("lock err")
		}

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			for range errChan {
				t.Error("errChan send a error")
			}
			wg.Done()
		}()

		err = unlock(ctx)
		if err != nil {
			t.Error("unlock error")
		}
		wg.Wait()
	})

	t.Run("sleep to TTL", func(t *testing.T) {
		unlock, errChan, err := l.TryLockWithAutoRenewal(ctx)
		if err != nil {
			t.Error("lock error")
		}

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			for range errChan {
				t.Error("errChan send a error")
			}
			wg.Done()
		}()

		<-time.After(6 * time.Second)

		err = unlock(ctx)
		if err != nil {
			t.Error("unlock error")
		}

		wg.Wait()
	})

	t.Run("context DeadlineExceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Nanosecond)
		defer cancel()

		unlock, errChan, err := l.TryLockWithAutoRenewal(ctx)
		if err != context.DeadlineExceeded {
			t.Error("error should be context.DeadlineExceeded")
		}

		if errChan != nil {
			t.Error("errChan should be nil")
		}

		if unlock != nil {
			t.Error("unlock should nil")
		}
	})

	t.Run("TryLock fail", func(t *testing.T) {
		unlock, errChan, err := l.TryLockWithAutoRenewal(ctx)
		if err != nil {
			t.Error("lock error")
		}

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			for v := range errChan {
				t.Error("errChan send a error:", v)
			}

			wg.Done()
		}()

		_, errChan, err = l.TryLockWithAutoRenewal(ctx)
		if err != ErrLockIsAcquired {
			t.Error("error should be ErrLockIsAcquired")
		}

		if errChan != nil {
			t.Error("errChan should be nil")
		}

		unlock(ctx)
		wg.Wait()
	})
}
