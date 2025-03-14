package lockerd

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

var clients []*redis.Client = []*redis.Client{
	redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", DB: 0}),
	redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", DB: 1}),
	redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", DB: 2}),
}

func TestRLocker_Lock(t *testing.T) {
	l, err := NewRLocker(clients, Options{Key: "k1"})
	if err != nil {
		t.Error("NewRLocker error:", err)
	}
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
		if !errors.Is(err, ErrLockKeyIsNotSet) {
			t.Error("error should contains ErrLockKeyIsNotSet")
		}
	})

	t.Run("context DeadlineExceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Nanosecond)
		defer cancel()

		unlock, err := l.Lock(ctx)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Error("error should be context.DeadlineExceeded")
		}

		if unlock != nil {
			t.Error("unlock should be nil")
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
					t.Error("lock error")
				} else {
					if l.GetValue() == "" {
						t.Error("value shouldn't be empty")
					}

					err = unlock(ctx)
					if err != nil {
						t.Error("unlock error")
					}
				}
			}()
		}

		wg.Wait()
	})
}

func TestRLocker_TryLock(t *testing.T) {
	l, err := NewRLocker(clients, Options{Key: "k1"})
	if err != nil {
		t.Error("NewRLocker error:", err)
	}
	ctx := context.Background()

	t.Run("lock and unlock", func(t *testing.T) {
		unlock, err := l.TryLock(ctx)
		if err != nil {
			t.Error("lock error")
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
		if !errors.Is(err, ErrLockKeyIsNotSet) {
			t.Error("lock should be expire")
		}
	})

	t.Run("context DeadlineExceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Nanosecond)
		defer cancel()

		unlock, err := l.TryLock(ctx)
		if err != ErrNotMoreThanHalfNodes {
			t.Error("error shoule be ErrNotMoreThanHalfNodes")
		}

		if l.GetValue() != "" {
			t.Error("_currentValue should be empty")
		}

		if unlock != nil {
			err = unlock(context.Background())
			if err != nil {
				t.Error("unlock error")
			}
		} else {
			t.Error("unlock is nil")
		}
	})

	t.Run("TryLock fail", func(t *testing.T) {
		unlock, err := l.TryLock(ctx)
		if err != nil {
			t.Error("lock error")
		}
		defer unlock(ctx)

		unlock, err = l.TryLock(ctx)
		if err != ErrNotMoreThanHalfNodes {
			t.Error("error should be ErrNotMoreThanHalfNodes")
		}

		if unlock != nil {
			err = unlock(ctx)
			if err != nil {
				t.Error("unlock error")
			}
		} else {
			t.Error("unlock is nil")
		}
	})
}

func TestRLocker_LockWithAutoRenewal(t *testing.T) {
	l, err := NewRLocker(clients, Options{Key: "k1"})
	if err != nil {
		t.Error("NewRLocker error")
	}
	ctx := context.Background()

	t.Run("lock and unlock", func(t *testing.T) {
		unlock, errChan, err := l.LockWithAutoRenewal(ctx)
		if err != nil {
			t.Error("lock error")
		}
		if errChan == nil {
			t.Error("errChan shouldn't be nil")
		}

		go func() {
			for err := range errChan {
				t.Error(err)
			}
		}()

		err = unlock(ctx)
		if err != nil {
			t.Error("unlock error")
		}
	})

	t.Run("context DeadlineExceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Nanosecond)
		defer cancel()

		unlock, errChan, err := l.LockWithAutoRenewal(ctx)
		if err != context.DeadlineExceeded {
			t.Error("error should be context.DeadlineExceeded")
		}
		if errChan != nil {
			t.Error("errChan should be nil")
		}
		if unlock != nil {
			t.Error("unlock should be nil")
		}
	})

	t.Run("sleep to TTL", func(t *testing.T) {
		unlock, errChan, err := l.LockWithAutoRenewal(ctx)
		if err != nil {
			t.Error("lock error")
		}
		defer unlock(ctx)

		go func(errChan <-chan error) {
			for range errChan {
				t.Error(err)
			}
		}(errChan)

		<-time.After(6 * time.Second)

		unlock, errChan, err = l.TryLockWithAutoRenewal(ctx)
		if err != ErrNotMoreThanHalfNodes {
			t.Error("lock error should be ErrNotMoreThanHalfNodes")
		}
		if errChan != nil {
			t.Error("errChan should be nil")
		}
	})

	t.Run("concurrency", func(t *testing.T) {
		wg := sync.WaitGroup{}

		for range 500 {
			wg.Add(1)

			go func() {
				defer wg.Done()

				unlock, errChan, err := l.LockWithAutoRenewal(ctx)
				if err != nil {
					t.Error("lock error")
				} else {
					if l.GetValue() == "" {
						t.Error("value shouldn't be empty before unlock")
					}

					err := unlock(ctx)
					if err != nil {
						t.Error("unlock error")
					}

					for range errChan {
						t.Error("errChan send a error")
					}
				}
			}()
		}

		wg.Wait()
	})
}

func TestRLocker_TryLockWithAutoRenewal(t *testing.T) {
	l, err := NewRLocker(clients, Options{Key: "k1"})
	if err != nil {
		t.Error("NewRLocker error")
	}
	ctx := context.Background()

	t.Run("lock and unlock", func(t *testing.T) {
		unlock, errChan, err := l.TryLockWithAutoRenewal(ctx)
		if err != nil {
			t.Error("lock error")
		}

		go func() {
			for range errChan {
				t.Error("errChan send a error")
			}
		}()

		err = unlock(ctx)
		if err != nil {
			t.Error("unlock error")
		}
	})

	t.Run("sleep to TTL", func(t *testing.T) {
		unlock, errChan, err := l.TryLockWithAutoRenewal(ctx)
		if err != nil {
			t.Error("lock error")
		}
		defer unlock(ctx)

		go func(errChan <-chan error) {
			for range errChan {
				t.Error("errChan send a error")
			}
		}(errChan)

		<-time.After(6 * time.Second)

		unlock, errChan, err = l.TryLockWithAutoRenewal(ctx)
		if err != ErrNotMoreThanHalfNodes {
			t.Error("error should be ErrNotMoreThanHalfNodes")
		}
		if errChan != nil {
			t.Error("errChan should be nil")
		}

		err = unlock(ctx)
		if err != nil {
			t.Error("unlock error")
		}
	})

	t.Run("context DeadlineExceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Nanosecond)
		defer cancel()

		unlock, errChan, err := l.TryLockWithAutoRenewal(ctx)
		if err != ErrNotMoreThanHalfNodes {
			t.Error("error should be ErrNotMoreThanHalfNodes")
		}
		if errChan != nil {
			t.Error("errChan should be nil")
		}
		err = unlock(ctx)
		if err != nil {
			t.Error("unlock error")
		}
	})

	t.Run("TryLock fail", func(t *testing.T) {
		unlock, errChan, err := l.TryLockWithAutoRenewal(ctx)
		if err != nil {
			t.Error("lock error")
		}

		go func(errChan <-chan error) {
			for range errChan {
				t.Error("errChan send a error")
			}
		}(errChan)

		defer unlock(ctx)

		unlock, errChan, err = l.TryLockWithAutoRenewal(ctx)
		if err != ErrNotMoreThanHalfNodes {
			t.Error("error should be ErrNotMoreThanHalfNodes")
		}

		if errChan != nil {
			t.Error("errChan should be nil")
		}

		err = unlock(ctx)
		if err != nil {
			t.Error("unlock error")
		}
	})
}
