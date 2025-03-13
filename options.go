package lockerd

import (
	"errors"
	"time"
)

type Options struct {
	// Key for identify lock.
	//
	// Default: No default value, Must specify.
	Key string

	// ValueGeneratorFunc produce value to identify ownership of
	// lock, make sure it generate unique value if you change it.
	//
	// Default: defaultValueGenerator
	ValueGeneratorFunc func() string

	// RetryInterval is the interval between each time try
	// to acquire lock in Lock and LockWithAutoRenewal method,
	// which is not include network IO cost.
	//
	// Default: 50ms
	RetryInterval time.Duration

	// TTL is the expiration send to redis to avoid dead
	// lock, any specified value smaller than 1s will be
	// replace with 1s.
	//
	// Default: 5s
	TTL time.Duration

	// AutoRenewalInterval is the interval of each renewal,
	// which is not include network IO cost. This option
	// only work when use auto renewal.
	//
	// Default: TTL / 2
	AutoRenewalInterval time.Duration

	// AutoRenewalTTL is the TTL that renewal set expiration
	// to, any specified value smaller than 1s will be replace
	// with 1s. This option only work when use auto renewal.
	//
	// Default: TTL
	AutoRenewalTTL time.Duration
}

func (opts Options) complete() (Options, error) {
	if opts.Key == "" {
		return opts, errors.New("Key must be specified")
	}

	if opts.ValueGeneratorFunc == nil {
		opts.ValueGeneratorFunc = defaultValueGenerator
	}

	if opts.RetryInterval == 0 {
		opts.RetryInterval = 50 * time.Millisecond
	}

	if opts.TTL == 0 {
		opts.TTL = 5 * time.Second
	}

	if opts.TTL < time.Second {
		opts.TTL = time.Second
	}

	if opts.AutoRenewalInterval == 0 {
		opts.AutoRenewalInterval = opts.TTL / 2
	}

	if opts.AutoRenewalTTL == 0 {
		opts.AutoRenewalTTL = opts.TTL
	}

	if opts.AutoRenewalTTL < time.Second {
		opts.AutoRenewalTTL = time.Second
	}

	if opts.AutoRenewalInterval > opts.AutoRenewalTTL {
		return opts, errors.New("AutoRenewalInterval longer than AutoRenewalTTL, unable to renewal")
	}

	if opts.TTL < opts.AutoRenewalInterval {
		return opts, errors.New("TTL is smaller than AutoRenewalInterval, unable to renewal")
	}

	return opts, nil
}
