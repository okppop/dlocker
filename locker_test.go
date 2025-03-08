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
	l := NewWithOptions(client, Options{
		Key: "key1",
	})

	wg := sync.WaitGroup{}

	for range 500 {
		wg.Add(1)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			unlock, err := l.Lock(ctx)
			if err != nil {
				t.Error("error happend in lock:", err)
			} else {
				err = unlock(ctx)
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
