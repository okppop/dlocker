package dlocker

import (
	"context"
	_ "embed"
	"errors"

	"github.com/redis/go-redis/v9"
)

// Locker is a distributed locker, config connection setting in
// *redis.Client, Options control locker's behavior
type Locker struct {
	client *redis.Client
	opts   Options
}

// New create a Locker instance with default Options, which
// you don't have to config anything besides redis client.
// default option see option.go: Options.complete
func New(client *redis.Client) *Locker {
	defaultOptions := Options{}
	defaultOptions = defaultOptions.complete()

	return &Locker{
		client: client,
		opts:   defaultOptions,
	}
}

// NewWithOptions create a Locker instance with opts, fields
// in opts not specified will replace with default value,
// see option.go: Options.complete
func NewWithOptions(client *redis.Client, opts Options) *Locker {
	opts = opts.complete()

	return &Locker{
		client: client,
		opts:   opts,
	}
}

// unlockScript is the lua script pass to redis EVAL
//
//go:embed lua/unlock.lua
var unlockScript string

// UnlockFunc represent the function to unlock, only return after
// (Locker).TryLock and (Locker).Lock
type UnlockFunc func(context.Context) error

// ErrLockerIsOccupied returned when the locker is occupied by others
var ErrLockerIsOccupied = errors.New("locker is occupied by others")

// TryLock try lock once, if the locker was occupied by others, return
// ErrLockerIsOccupied error
func (l *Locker) TryLock(ctx context.Context) (UnlockFunc, error) {
	return l.lock(ctx)
}

func (l *Locker) lock(ctx context.Context) (UnlockFunc, error) {
	value := l.opts.ValueGeneratorFunc()

	ok, err := l.client.SetNX(ctx, l.opts.Key, value, l.opts.TTL).Result()
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, ErrLockerIsOccupied
	}

	return func(ctx context.Context) error {
		// TODO: maybe some edge condition
		_, err := l.client.Eval(ctx, unlockScript, []string{l.opts.Key}, value).Result()
		if err != nil {
			return err
		}

		return nil
	}, nil
}
