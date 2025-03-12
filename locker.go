package lockerd

import (
	"context"
	_ "embed"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// unlockScript is the lua script pass to redis EVAL
	// for unlock.
	//
	//go:embed lua/unlock.lua
	unlockScript string

	// renewalScript is the lua script pass to redis EVAL
	// for renewal.
	//
	//go:embed lua/renewal.lua
	renewalScript string
)

var (
	// ErrLockIsAcquired returned when lock is acquired by
	// others.
	ErrLockIsAcquired = errors.New("lock is acquired by others, therefor can't be lock, unlock or renewal")

	// ErrLockKeyIsNotSet returned when no one acquire
	// lock, key is not set, therefor can't be unlock or
	// renewal.
	ErrLockKeyIsNotSet = errors.New("lock key is not set, can't be unlock or renewal")
)

// UnlockFunc represent the function to unlock.
type UnlockFunc func(context.Context) error

// Locker represent distributed lock for single redis node.
type Locker struct {
	// client provides redis connections and config.
	client *redis.Client
	// opts control lock's behavior.
	opts LockerOptions
	// internal field
	_currentValue string
}

// NewLocker create a Locker with client and opts, fields
// in opts weren't specified will be replaced with default
// value, besides Key which must specify, see LockerOptions
// and LockerOptions.complete.
func NewLocker(client *redis.Client, opts LockerOptions) (*Locker, error) {
	opts, err := opts.complete()
	if err != nil {
		return nil, err
	}

	return &Locker{
		client: client,
		opts:   opts,
	}, nil
}

// Lock keep trying to acquire lock until ctx is canceled or
// deadline exceeded, ensure passing a proper ctx to avoid
// infinite retries.
//
// To unlock lock, call UnlockFunc that returned.
//
// Retry interval between two attempts according to
// Locker.opts.RetryInterval.
func (l *Locker) Lock(ctx context.Context) (UnlockFunc, error) {
	value := l.opts.ValueGeneratorFunc()

	// Go doesn't support tail call optimization, see:
	// https://groups.google.com/g/golang-nuts/c/0oIZPHhrDzY/m/2nCpUZDKZAAJ
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			unlock, err := l.tryLock(ctx, value)

			switch err {
			// key exists, SET fail.
			case ErrLockIsAcquired:
				<-time.After(l.opts.RetryInterval)
				continue
			case nil:
				return unlock, nil
			// connection error, or ctx done during
			// Locker.tryLock.
			default:
				return nil, err
			}
		}
	}
}

// TryLock try to acquire lock once, if lock was acquired by
// others, return (nil, ErrLockIsAcquired).
//
// To unlock lock, call UnlockFunc that returned.
func (l *Locker) TryLock(ctx context.Context) (UnlockFunc, error) {
	return l.tryLock(ctx, l.opts.ValueGeneratorFunc())
}

// LockWithAutoRenewal keep trying to acquire lock until ctx
// is canceled or deadline exceeded, ensure passing a proper
// ctx to avoid infinite retries.
//
// To unlock lock, call UnlockFunc that returned.
//
// Retry interval between two attempts according to
// Locker.opts.RetryInterval.
//
// If success, a goroutinue will be started to auto renewal
// the lock for avoiding lock expire when reach
// Locker.opts.TTL, the read-only channel returned will
// pass error which happend in auto renewal goroutine.
func (l *Locker) LockWithAutoRenewal(ctx context.Context) (UnlockFunc, <-chan error, error) {
	value := l.opts.ValueGeneratorFunc()

	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
			unlock, errChan, err := l.tryLockWithAutoRenewal(ctx, value)

			switch err {
			// key exists, SET fail.
			case ErrLockIsAcquired:
				<-time.After(l.opts.RetryInterval)
				continue
			case nil:
				return unlock, errChan, nil
			// connection error, or ctx done during
			// Locker.tryLock.
			default:
				return nil, nil, err
			}
		}
	}
}

// TryLockWithAutoRenewal try to acquire lock once, if lock
// was acquired by others, return (nil, ErrLockIsAcquired).
//
// To unlock lock, call UnlockFunc that returned.
//
// If success, a goroutinue will be started to auto renewal
// the lock for avoiding lock expire when reach
// Locker.opts.TTL, the read-only channel returned will
// pass error which happend in auto renewal goroutine.
func (l *Locker) TryLockWithAutoRenewal(ctx context.Context) (UnlockFunc, <-chan error, error) {
	return l.tryLockWithAutoRenewal(ctx, l.opts.ValueGeneratorFunc())
}

func (l *Locker) tryLock(ctx context.Context, value string) (UnlockFunc, error) {
	ok, err := l.client.SetNX(ctx, l.opts.Key, value, l.opts.TTL).Result()
	// connection error, or ctx done.
	if err != nil {
		return nil, err
	}

	// key exists, SET fail.
	if !ok {
		return nil, ErrLockIsAcquired
	}

	l._currentValue = value

	return l.pUnlockFunc(value), nil
}

func (l *Locker) tryLockWithAutoRenewal(ctx context.Context, value string) (UnlockFunc, <-chan error, error) {
	_, err := l.tryLock(ctx, value)
	if err != nil {
		return nil, nil, err
	}

	// for control auto renewal goroutinue's life cycle.
	autoRenewalCtx, cancelAutoRenewal := context.WithCancel(context.Background())
	errChan := make(chan error, 1)

	go l.autoRenewal(autoRenewalCtx, value, errChan)

	return l.pUnlockFuncWithAutoRenewal(value, cancelAutoRenewal), errChan, nil
}

// pUnlockFunc return UnlockFunc for Locker.Lock and
// Locker.TryLock.
func (l *Locker) pUnlockFunc(value string) UnlockFunc {
	return func(ctx context.Context) error {
		l._currentValue = ""

		return l.unlock(ctx, value)
	}
}

// pUnlockFuncWithAutoRenewal return UnlockFunc for
// Locker.LockWithAutoRenewal and
// Locker.TryLockWithAutoRenewal.
func (l *Locker) pUnlockFuncWithAutoRenewal(value string, cancelAutoRenewal context.CancelFunc) UnlockFunc {
	return func(ctx context.Context) error {
		l._currentValue = ""
		cancelAutoRenewal()

		return l.unlock(ctx, value)
	}
}

func (l *Locker) unlock(ctx context.Context, value string) error {
	res, err := l.client.Eval(ctx, unlockScript, []string{l.opts.Key}, value).Result()
	// GET key returns nil.
	if err == redis.Nil {
		return ErrLockKeyIsNotSet
	}

	// connection error, or ctx done.
	if err != nil {
		return err
	}

	switch res.(type) {
	// redis returns the number of keys it successfully
	// delete.
	//
	// if return 1, everything is fine.
	//
	// if return 0, barely happend, maybe key expired
	// during unlock, I believe it's fine, since the key
	// must doesn't exist now.
	case int64:
		return nil
	// redis doesn't return a number means lock is acquired
	// by others.
	default:
		return ErrLockIsAcquired
	}
}

// autoRenewal renewals lock periodicity, if error happend,
// send to errChan, close errChan and stop.
//
// Action interval respect Locker.opts.AutoRenewalInterval,
// renewal TTL set to Locker.opts.AutoRenewalTTL.
func (l *Locker) autoRenewal(ctx context.Context, value string, errChan chan<- error) {
	defer close(errChan)
	for {
		select {
		// unlock was called.
		case <-ctx.Done():
			return
		case <-time.After(l.opts.AutoRenewalInterval):
			res, err := l.client.Eval(ctx, renewalScript, []string{l.opts.Key}, value, int64(l.opts.AutoRenewalTTL/time.Second)).Result()

			switch err {
			// unlock was called during renewal.
			case context.Canceled, context.DeadlineExceeded:
				return
			// key is not set, maybe expired.
			case redis.Nil:
				errChan <- ErrLockKeyIsNotSet
				return
			case nil:
				switch res.(type) {
				// renewal success.
				case int64:
					continue
				// lock is acquired by others, take this
				// error seriously.
				default:
					errChan <- ErrLockIsAcquired
					return
				}
			default:
				// probably network issue, check error for
				// details.
				errChan <- err
				return
			}
		}
	}
}

// GetValue should only be called when you are acquired the
// lock, otherwise you may get other owner's value instead
// of empty string.
func (l *Locker) GetValue() string {
	return l._currentValue
}

func (l Locker) GetLockerOptionsCopy() LockerOptions {
	return l.opts
}
