# lockerd
[![Go Reference](https://pkg.go.dev/badge/github.com/okppop/lockerd.svg)](https://pkg.go.dev/github.com/okppop/lockerd)

lockerd is a distributed lock library for Go, based on [redis](https://redis.io/) and [go-redis](https://github.com/redis/go-redis), easy to use, with great documentation.

Offering auto renewal(watch dog mode), redlock for different scenarios.

## Install
```shell
go get github.com/okppop/lockerd
go get github.com/redis/go-redis/v9
```

## Examples

Some simple example below for you to start, for more information, see Documentation.

### Single redis node
```go
package main

import (
	"context"
	"time"

	"github.com/okppop/lockerd"
	"github.com/redis/go-redis/v9"
)

func main() {
	// init lock
	client := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "",
		DB:       0,
	})
	l, err := lockerd.NewLocker(client, lockerd.Options{
		// specify Key at least
		Key: "lock-k1",
	})
	if err != nil {
		panic(err)
	}

	// 1. without auto renewal
	//
	// A empty context may cause infinite retries
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	unlock, err := l.Lock(ctx)
	if err != nil {
		//
	}

	err = unlock(ctx)
	if err != nil {
		//
	}

	// 2. with auto renewal(watch dog mode)
	//
	// errChan will pass the error happend in auto renewal,
	// you may want to ignore it:
	// unlock, _, err := l.LockWithAutoRenewal(ctx)
	unlock, errChan, err := l.LockWithAutoRenewal(ctx)
	if err != nil {
		//
	}

	go func(errChan <-chan error) {
		for range errChan {
			//
		}
	}(errChan)

	err = unlock(ctx)
	if err != nil {
		//
	}

	// 3. error checking
	//
	unlock, errChan, err = l.LockWithAutoRenewal(ctx)
	if err != nil {
		switch err {
		case context.Canceled, context.DeadlineExceeded:
			//
		default:
			//
		}
	}

	go func(errChan <-chan error) {
		for err := range errChan {
			switch err {
			case lockerd.ErrLockKeyIsNotSet:
				//
			case lockerd.ErrLockIsAcquired:
				//
			default:
				//
			}
		}
	}(errChan)

	err = unlock(ctx)
	if err != nil {
		switch err {
		case lockerd.ErrLockKeyIsNotSet:
			//
		case lockerd.ErrLockIsAcquired:
			//
		default:
			//
		}
	}
}
```

### Multiple redis nodes
```go
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/okppop/lockerd"
	"github.com/redis/go-redis/v9"
)

func main() {
	// init lock
	l, err := lockerd.NewRLocker(
		[]*redis.Client{
			redis.NewClient(&redis.Options{
				Addr:     "192.168.0.2: 6379",
				Password: "",
				DB:       0,
			}),
			redis.NewClient(&redis.Options{
				Addr:     "192.168.0.3: 6379",
				Password: "",
				DB:       0,
			}),
			redis.NewClient(&redis.Options{
				Addr:     "192.168.0.4: 6379",
				Password: "",
				DB:       0,
			}),
		},
		lockerd.Options{
			Key: "lock-k1",
		},
	)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 1. redlock without auto renewal
	unlock, err := l.Lock(ctx)
	if err != nil {
		fmt.Println(err)
		// call unlock even Lock fail
		err = unlock(ctx)
		if err != nil {
			fmt.Println(err)
		}

		return
	}

	err = unlock(ctx)
	if err != nil {
		fmt.Println(err)
	}

	// 2. redlock with auto renewal(watch dog mode)
	unlock, errChan, err := l.LockWithAutoRenewal(ctx)
	if err != nil {
		fmt.Println(err)

		err = unlock(ctx)
		if err != nil {
			fmt.Println(err)
		}

		return
	}

	go func(errChan <-chan error) {
		for err := range errChan {
			fmt.Println(err)
		}
	}(errChan)

	err = unlock(ctx)
	if err != nil {
		fmt.Println(err)
	}

	// 3. error checking
	unlock, errChan, err = l.LockWithAutoRenewal(ctx)
	if err != nil {
		// when err != nil, err always is lockerd.ErrNotMoreThanHalfNodes in here
		if err == lockerd.ErrNotMoreThanHalfNodes {
			fmt.Println(err)
		}

		err = unlock(ctx)
		if err != nil {
			//
		}

		return
	}

	go func(errChan <-chan error) {
		for err := range errChan {
			if errors.Is(err, lockerd.ErrLockKeyIsNotSet) {
				//
			}
			if errors.Is(err, lockerd.ErrLockIsAcquired) {
				//
			}

			// error sent from errChan will contains redis nodes info
			fmt.Println(err)
		}
	}(errChan)

	err = unlock(ctx)
	if err != nil {
		if errors.Is(err, lockerd.ErrLockKeyIsNotSet) {
			//
		}
		if errors.Is(err, lockerd.ErrLockIsAcquired) {
			//
		}

		// error from unlock also contains redis nodes info
		fmt.Println(err)
	}
}
```

## Documentation

- [Go Reference](https://pkg.go.dev/github.com/okppop/lockerd)