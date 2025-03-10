package dlocker

import (
	"time"

	"github.com/google/uuid"
)

type Options struct {
	// Key for identify each lock.
	//
	// Default is a random UUID.
	Key string

	// ValueGeneratorFunc produce value to identify ownership
	// of lock, make sure it generate unique value if you
	// change it.
	//
	// Default is defaultGenerator().
	ValueGeneratorFunc func() string

	// RetryInterval is the interval between each time try to lock
	// in *Locker.Lock(), which is not include network IO cost.
	//
	// Default is 50ms.
	RetryInterval time.Duration

	// TTL is the expiration send to redis to avoid dead lock,
	// minimal supported value by redis is 1ms, therefor any value
	// smaller than 1ms will be replace with 1ms.
	//
	// Default is 6s.
	TTL time.Duration

	// DisableAutoRenewal control whether use auto renewal, which
	// is the mechanism to auto renewal TTL if the lock isn't
	// unlock before reach TTL.
	//
	// Default is false.
	DisableAutoRenewal bool

	// AutoRenewalInterval is the interval of renewal action,
	// which is not include network IO cost. This option only
	// work when DisableAutoRenewal is false.
	//
	// Default is half of TTL.
	AutoRenewalInterval time.Duration

	// AutoRenewalTTL is the TTL that renewal set expire time(TTL)
	// to, minimal supported value by redis is 1ms, therefor any
	// value smaller than 1ms will be replace with 1ms. This
	// option only work when DisableAutoRenewal is false.
	//
	// Default is equal to TTL.
	AutoRenewalTTL time.Duration
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
		opt.TTL = 6 * time.Second
	}

	if opt.TTL < time.Millisecond {
		opt.TTL = time.Millisecond
	}

	if !opt.DisableAutoRenewal {
		if opt.AutoRenewalInterval == 0 {
			opt.AutoRenewalInterval = opt.TTL / 2
		}

		if opt.AutoRenewalTTL == 0 {
			opt.AutoRenewalTTL = opt.TTL
		}

		if opt.AutoRenewalTTL < time.Millisecond {
			opt.AutoRenewalTTL = time.Millisecond
		}
	}

	return opt
}
