package api

import (
	"context"
	"errors"
	"sync"
	"time"
)

// errNegativeCache reports a load suppressed by a recent failure with no
// stale value to serve
var errNegativeCache = errors.New("stats refresh failed recently, retry later")

// staleCacheLoadTimeout bounds the detached refresh query
const staleCacheLoadTimeout = 60 * time.Second

// staleCache is a single-flight TTL cache: concurrent stale reads trigger
// exactly one load, a failed refresh serves the last good value when one
// exists, and a short negative window suppresses retry storms when there is
// no stale value to fall back to.
type staleCache[T any] struct {
	mu      sync.Mutex
	cond    *sync.Cond
	val     *T
	at      time.Time
	failAt  time.Time
	loading bool
}

func newStaleCache[T any]() *staleCache[T] {
	c := &staleCache[T]{}
	c.cond = sync.NewCond(&c.mu)
	return c
}

// get returns the cached value, refreshing it through load when stale. Only
// one caller loads at a time; the rest wait for its result. When the load
// fails, a previously cached value is served instead, and for errTTL after a
// failure no new load is attempted.
func (c *staleCache[T]) get(ctx context.Context, ttl, errTTL time.Duration, load func(context.Context) (*T, error)) (*T, error) {
	c.mu.Lock()
	for {
		if c.val != nil && time.Since(c.at) < ttl {
			val := c.val
			c.mu.Unlock()
			return val, nil
		}
		if !c.loading {
			break
		}
		c.cond.Wait()
	}
	if time.Since(c.failAt) < errTTL {
		val := c.val
		c.mu.Unlock()
		if val != nil {
			return val, nil
		}
		return nil, errNegativeCache
	}
	c.loading = true
	c.mu.Unlock()

	// the loading flag must clear even when load panics (net/http recovers
	// panics per request, so a stuck flag would strand every later caller
	// in cond.Wait forever)
	defer func() {
		c.mu.Lock()
		c.loading = false
		c.cond.Broadcast()
		c.mu.Unlock()
	}()

	// the load outlives the leader's request: a client disconnect must not
	// abort the shared refresh or count as a backend failure
	loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), staleCacheLoadTimeout)
	defer cancel()
	val, err := load(loadCtx)

	c.mu.Lock()
	if err == nil {
		c.val = val
		c.at = time.Now()
		c.failAt = time.Time{}
	} else {
		if !errors.Is(err, context.Canceled) {
			c.failAt = time.Now()
		}
		if c.val != nil {
			val, err = c.val, nil
		}
	}
	c.mu.Unlock()
	return val, err
}
