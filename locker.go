package dlocker

import (
	"context"
	_ "embed"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// unlockScript is the lua script pass to redis EVAL for unlock.
//
//go:embed lua/unlock.lua
var unlockScript string

// renewalScript is the lua script pass to redis EVAL for renewal.
//
//go:embed lua/renewal.lua
var renewalScript string

// renewalPScript is similar to renewalScript, for the
// conditions which Options.AutoRenewalTTL smaller than 1s.
//
//go:embed lua/renewal_p.lua
var renewalPScript string

// UnlockFunc represent the function to unlock, only return after
// *Locker.TryLock() and *Locker.Lock().
type UnlockFunc func(context.Context) error

// ErrLockIsAcquired returned when the lock is acquired by
// others.
var ErrLockIsAcquired = errors.New("lock is acquired by others")

// Locker is a distributed lock, config connection setting in
// *redis.Client, Options control locker's behavior.
type Locker struct {
	client        *redis.Client
	opts          Options
	currentValue  string
	renewalScript string
	renewalTTLArg int64
}

// New create a Locker with default Options, which is not
// recommend.
//
// Default option see Options.complete().
func New(client *redis.Client) *Locker {
	defaultOptions := Options{}
	defaultOptions = defaultOptions.complete()

	return &Locker{
		client:        client,
		opts:          defaultOptions,
		renewalScript: renewalScript,
		renewalTTLArg: int64(defaultOptions.AutoRenewalTTL / time.Second),
	}
}

// NewWithOptions create a Locker with opts, fields in opts
// not specified will replace with default value.
//
// Default option see Options.complete().
func NewWithOptions(client *redis.Client, opts Options) *Locker {
	opts = opts.complete()

	var script string
	var ttlArg int64

	if !opts.DisableAutoRenewal {
		if usePrecise(opts.AutoRenewalTTL) {
			script = renewalPScript
			ttlArg = int64(opts.AutoRenewalTTL / time.Millisecond)
		} else {
			script = renewalScript
			ttlArg = int64(opts.AutoRenewalTTL / time.Second)
		}
	}

	return &Locker{
		client:        client,
		opts:          opts,
		renewalScript: script,
		renewalTTLArg: ttlArg,
	}
}

// TryLock try to acquire lock once, if the lock was acquired by
// others, return (nil, ErrLockIsAcquired).
func (l *Locker) TryLock(ctx context.Context) (UnlockFunc, error) {
	return l.lockWithValue(ctx, l.opts.ValueGeneratorFunc())
}

// Lock keep trying to acquire lock until ctx is canceled or
// deadline exceeded, ensure to use a proper context to avoid
// infinite retries.
//
// Retry interval between two attempts according to
// *Locker.opts.RetryInterval.
func (l *Locker) Lock(ctx context.Context) (UnlockFunc, error) {
	value := l.opts.ValueGeneratorFunc()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			unlock, err := l.lockWithValue(ctx, value)
			if err == ErrLockIsAcquired {
				<-time.After(l.opts.RetryInterval)
				break
			}

			if err != nil {
				return nil, err
			}

			return unlock, nil
		}
	}
}

func (l *Locker) lockWithValue(ctx context.Context, value string) (UnlockFunc, error) {
	ok, err := l.client.SetNX(ctx, l.opts.Key, value, l.opts.TTL).Result()
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, ErrLockIsAcquired
	}

	l.currentValue = value

	autoRenewalCtx, cancelAutoRenewal := context.WithCancel(context.Background())
	go l.autoRenewal(autoRenewalCtx, value)

	return func(ctx context.Context) error {
		l.currentValue = ""
		cancelAutoRenewal()

		// TODO: maybe some edge condition
		_, err := l.client.Eval(ctx, unlockScript, []string{l.opts.Key}, value).Result()
		if err != nil {
			return err
		}

		return nil
	}, nil
}

func (l *Locker) autoRenewal(ctx context.Context, value string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(l.opts.AutoRenewalInterval):
			_, err := l.client.Eval(ctx, l.renewalScript, []string{l.opts.Key}, value, l.renewalTTLArg).Result()

			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			// TODO: handle error
		}
	}
}

func (l *Locker) GetKey() string {
	return l.opts.Key
}

func (l *Locker) GetRetryInterval() time.Duration {
	return l.opts.RetryInterval
}

func (l *Locker) GetTTL() time.Duration {
	return l.opts.TTL
}

// GetValue should only be called when you are acquired the lock,
// otherwise you may get other owner's value instead of empty
// string.
func (l *Locker) GetValue() string {
	return l.currentValue
}

func usePrecise(d time.Duration) bool {
	return d < time.Second || d%time.Second != 0
}
