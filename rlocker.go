package lockerd

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrNotMoreThanHalfNodes returns when unable acquire more
	// than half of redis nodes, only returned from TryLock*.
	ErrNotMoreThanHalfNodes = errors.New("unable acquire more than half of redis nodes, acquire lock fail therefor")
)

// RLocker represent distributed lock for multiple redis nodes,
// implement redlock.
type RLocker struct {
	// clients provides a list of redis nodes.
	clients []*redis.Client
	// opts control lock's behavior.
	opts Options
	// internal field
	_currentValue string
	_minAcquired  int
	_maxAcquired  int
}

// NewRLocker create a RLocker with cliens slice and opts, fields
// in opts weren't specified will be replaced with default value,
// besides Key which must specify, see Options and
// Options.complete.
//
// clients at least have three client.
func NewRLocker(clients []*redis.Client, opts Options) (*RLocker, error) {
	n := len(clients)
	if n < 3 {
		return nil, errors.New("the number of clients is smaller than 3")
	}

	c := make([]*redis.Client, n)
	copy(c, clients)

	opts, err := opts.complete()
	if err != nil {
		return nil, err
	}

	return &RLocker{
		clients:      c,
		opts:         opts,
		_minAcquired: n/2 + 1,
		_maxAcquired: n,
	}, nil
}

// Lock keep trying to acquire lock until ctx is canceled or
// deadline exceeded, ensure passing a proper ctx to avoid
// infinite retries.
//
// To unlock lock, call UnlockFunc that returned.
//
// Retry interval between two attempts according to
// RLocker.opts.RetryInterval.
//
// Only possible error returned is context.Canceled and
// context.DeadlineExceeded.
func (l *RLocker) Lock(ctx context.Context) (UnlockFunc, error) {
	value := l.opts.ValueGeneratorFunc()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			unlock, err := l.tryLock(ctx, value)
			if err == ErrNotMoreThanHalfNodes {
				<-time.After(l.opts.RetryInterval)
				continue
			}

			return unlock, nil
		}
	}
}

// TryLock try to acquire lock once, return (nil,
// ErrNotMoreThanHalfNodes) when can't acquire lock at least half
// of redis nodes.
//
// To unlock lock, call UnlockFunc that returned.
func (l *RLocker) TryLock(ctx context.Context) (UnlockFunc, error) {
	return l.tryLock(ctx, l.opts.ValueGeneratorFunc())
}

// LockWithAutoRenewal keep trying to acquire lock until ctx is
// canceled or deadline exceeded, ensure passing a proper ctx
// to avoid infinite retries.
//
// To unlock lock, call UnlockFunc that returned.
//
// Retry interval between two attempts according to
// RLocker.opts.RetryInterval.
//
// Only possible error returned is context.Canceled and
// context.DeadlineExceeded.
//
// If success, multiple goroutinues will be started to auto
// renewal the lock for avoiding lock expire when reach
// RLocker.opts.TTL, the read-only channel returned will pass
// error which happend in auto renewal goroutinues, the error
// returned from channel was wrapped or joined with redis node's
// info, therefor use errors.Is to estimate what happend rather
// than err == Errxxx.
func (l *RLocker) LockWithAutoRenewal(ctx context.Context) (UnlockFunc, <-chan error, error) {
	value := l.opts.ValueGeneratorFunc()

	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
			unlock, errChan, err := l.tryLockWithAutoRenewal(ctx, value)
			if err == ErrNotMoreThanHalfNodes {
				<-time.After(l.opts.RetryInterval)
				continue
			}

			return unlock, errChan, nil
		}
	}
}

// TryLockWithAutoRenewal try to acquire lock once, return (nil,
// ErrNotMoreThanHalfNodes) when can't acquire lock at least half
// of redis nodes.
//
// If success, multiple goroutinues will be started to auto
// renewal the lock for avoiding lock expire when reach
// RLocker.opts.TTL, the read-only channel returned will pass
// error which happend in auto renewal goroutinues, the error
// returned from channel was wrapped or joined with redis node's
// info, therefor use errors.Is to estimate what happend rather
// than err == Errxxx.
func (l *RLocker) TryLockWithAutoRenewal(ctx context.Context) (UnlockFunc, <-chan error, error) {
	return l.tryLockWithAutoRenewal(ctx, l.opts.ValueGeneratorFunc())
}

func (l *RLocker) tryLock(ctx context.Context, value string) (UnlockFunc, error) {
	successClients := make([]*redis.Client, 0, l._maxAcquired)
	var successClientMutex sync.Mutex

	var wg sync.WaitGroup
	wg.Add(l._maxAcquired)

	for _, client := range l.clients {
		go func(client *redis.Client) {
			defer wg.Done()

			ok, err := client.SetNX(ctx, l.opts.Key, value, l.opts.TTL).Result()
			// connection error, or ctx done.
			if err != nil {
				return
			}

			// key exists, SET fail.
			if !ok {
				return
			}

			successClientMutex.Lock()
			defer successClientMutex.Unlock()

			successClients = append(successClients, client)
		}(client)
	}

	wg.Wait()

	if len(successClients) >= l._minAcquired {
		l._currentValue = value
		return l.pUnlockFunc(successClients, value), nil
	} else {
		l.unlock(ctx, successClients, value)
		return nil, ErrNotMoreThanHalfNodes
	}
}

func (l *RLocker) tryLockWithAutoRenewal(ctx context.Context, value string) (UnlockFunc, <-chan error, error) {
	successClients := make([]*redis.Client, 0, l._maxAcquired)
	var successClientMutex sync.Mutex

	var wg sync.WaitGroup
	wg.Add(l._maxAcquired)

	for _, client := range l.clients {
		go func(client *redis.Client) {
			defer wg.Done()

			ok, err := client.SetNX(ctx, l.opts.Key, value, l.opts.TTL).Result()
			// connection err, or ctx done.
			if err != nil {
				return
			}

			// key exists, SET fail.
			if !ok {
				return
			}

			successClientMutex.Lock()
			defer successClientMutex.Unlock()

			successClients = append(successClients, client)
		}(client)
	}

	wg.Wait()

	if len(successClients) >= l._minAcquired {
		l._currentValue = value

		// for control auto renewal goroutinue's life cycle.
		autoRenewalCtx, cancelAutoRenewal := context.WithCancel(context.Background())
		errChan := make(chan error, len(successClients))

		go l.autoRenewal(autoRenewalCtx, successClients, value, errChan)

		return l.pUnlockFuncWithAutoRenewal(successClients, value, cancelAutoRenewal), errChan, nil
	} else {
		l.unlock(ctx, successClients, value)
		return nil, nil, ErrNotMoreThanHalfNodes
	}
}

func (l *RLocker) pUnlockFunc(clients []*redis.Client, value string) UnlockFunc {
	return func(ctx context.Context) error {
		l._currentValue = ""

		return l.unlock(ctx, clients, value)
	}
}

func (l *RLocker) pUnlockFuncWithAutoRenewal(clients []*redis.Client, value string, cancelAutoRenewal context.CancelFunc) UnlockFunc {
	return func(ctx context.Context) error {
		l._currentValue = ""
		cancelAutoRenewal()

		return l.unlock(ctx, clients, value)
	}
}

func (l *RLocker) unlock(ctx context.Context, clients []*redis.Client, value string) error {
	var finalErr error
	var finalErrMutex sync.Mutex

	var wg sync.WaitGroup
	wg.Add(len(clients))

	for _, client := range clients {
		go func(client *redis.Client) {
			defer wg.Done()

			res, err := client.Eval(ctx, unlockScript, []string{l.opts.Key}, value).Result()
			// GET key returns nil.
			if err == redis.Nil {
				err = fmt.Errorf("%s: %w", client.String(), ErrLockKeyIsNotSet)

				finalErrMutex.Lock()
				defer finalErrMutex.Unlock()

				finalErr = errors.Join(finalErr, err)
				return
			}

			// connection error, or ctx done.
			if err != nil {
				err = fmt.Errorf("%s: %w", client.String(), err)

				finalErrMutex.Lock()
				defer finalErrMutex.Unlock()

				finalErr = errors.Join(finalErr, err)
				return
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
				return
			// redis doesn't return a number means lock is
			// acquired by others.
			default:
				err = fmt.Errorf("%s: %w", client.String(), ErrLockIsAcquired)

				finalErrMutex.Lock()
				defer finalErrMutex.Unlock()

				finalErr = errors.Join(finalErr, err)
				return
			}
		}(client)
	}

	wg.Wait()

	return finalErr
}

func (l *RLocker) autoRenewal(ctx context.Context, clients []*redis.Client, value string, errChan chan<- error) {
	defer close(errChan)

	var wg sync.WaitGroup
	wg.Add(len(clients))

	for _, client := range clients {
		go func(client *redis.Client) {
			defer wg.Done()

			for {
				select {
				// unlock was called
				case <-ctx.Done():
					return
				case <-time.After(l.opts.AutoRenewalInterval):
					res, err := client.Eval(ctx, renewalScript, []string{l.opts.Key}, value, int64(l.opts.AutoRenewalTTL/time.Second)).Result()

					switch err {
					// unlock was called during renewal.
					case context.Canceled, context.DeadlineExceeded:
						return
					// Key is not set, maybe expired.
					case redis.Nil:
						errChan <- fmt.Errorf("%s: %w", client.String(), ErrLockKeyIsNotSet)
						return
					case nil:
						switch res.(type) {
						// renewal success.
						case int64:
							continue
						// lock is acquired by others, take this
						// error seriously.
						default:
							errChan <- fmt.Errorf("%s: %w", client.String(), ErrLockIsAcquired)
							return
						}
					// probably network issue, check error for
					// details.
					default:
						errChan <- fmt.Errorf("%s: %w", client.String(), err)
						return
					}
				}
			}
		}(client)
	}

	wg.Wait()
}

// GetValue should only be called when you are acquired the
// lock, otherwise you may get other owner's value instead
// of empty string.
func (l *RLocker) GetValue() string {
	return l._currentValue
}

func (l RLocker) GetOptionsCopy() Options {
	return l.opts
}
