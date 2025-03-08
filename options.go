package dlocker

import (
	"time"

	"github.com/google/uuid"
)

type Options struct {
	// Key for identify each locker
	Key string

	// ValueGeneratorFunc produce value to identify ownership of lock,
	// make sure it generate unique value if you change it
	ValueGeneratorFunc func() string

	// RetryInterval is the interval between each time try to lock
	// in *Locker.Lock()
	RetryInterval time.Duration

	// TTL is the expiration send to redis to avoid dead lock,
	// minimal supported value by redis is 1ms, Options.complete
	// will trim any value smaller than 1ms to 1ms
	TTL time.Duration
}

func (opt Options) complete() Options {
	if opt.Key == "" {
		opt.Key = uuid.New().String()
	}

	if opt.ValueGeneratorFunc == nil {
		opt.ValueGeneratorFunc = defaultGenerator
	}

	if opt.RetryInterval == 0 {
		opt.RetryInterval = 50 * time.Millisecond
	}

	if opt.TTL == 0 {
		opt.TTL = time.Second
	}

	if opt.TTL < time.Millisecond {
		opt.TTL = time.Millisecond
	}

	return opt
}
