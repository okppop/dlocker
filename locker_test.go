package dlocker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

var client *redis.Client

func TestTryLock(t *testing.T) {
	ctx := context.Background()

	lockerA := NewWithOptions(client, Options{Key: "key1", TTL: time.Hour})

	unlock, err := lockerA.TryLock(ctx)
	if err != nil {
		t.Error("try lock error:", err)
	}

	err = unlock(ctx)
	if err != nil {
		t.Error("unlock error:", err)
	}
}

func TestLock(t *testing.T) {
	l := New(client)

	wg := sync.WaitGroup{}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for range 500 {
		wg.Add(1)
		go func() {
			unlock, err := l.Lock(ctx)
			if err != nil {
				t.Error("error happend in lock:", err)
			} else {
				if l.GetValue() == "" {
					t.Error("value should not be empty before unlock")
				}

				err := unlock(ctx)
				if err != nil {
					t.Error("error happend in unlock:", err)
				}
			}

			wg.Done()
		}()
	}

	wg.Wait()
}

func init() {
	c := redis.NewClient(&redis.Options{
		Network:  "tcp",
		Addr:     "127.0.0.1:6379",
		Password: "",
		DB:       0,
	})

	client = c
}
