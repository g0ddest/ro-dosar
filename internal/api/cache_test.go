package api

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStaleCache_SingleFlight(t *testing.T) {
	c := newStaleCache[int]()
	var calls atomic.Int32
	load := func(context.Context) (*int, error) {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond)
		v := 42
		return &v, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := c.get(context.Background(), time.Minute, time.Second, load)
			if err != nil || *v != 42 {
				t.Errorf("get: %v %v", v, err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Errorf("expected exactly 1 load, got %d", calls.Load())
	}
}

func TestStaleCache_ServesStaleOnError(t *testing.T) {
	c := newStaleCache[int]()
	v := 42
	ok := func(context.Context) (*int, error) { return &v, nil }
	fail := func(context.Context) (*int, error) { return nil, errors.New("db down") }

	if got, err := c.get(context.Background(), 0, 0, ok); err != nil || *got != 42 {
		t.Fatalf("prime: %v %v", got, err)
	}
	// ttl 0 => immediately stale; the failing refresh must serve the old value
	got, err := c.get(context.Background(), 0, 0, fail)
	if err != nil || got == nil || *got != 42 {
		t.Errorf("expected stale 42 on refresh failure, got %v %v", got, err)
	}
}

func TestStaleCache_NegativeWindowSuppressesRetries(t *testing.T) {
	c := newStaleCache[int]()
	var calls atomic.Int32
	fail := func(context.Context) (*int, error) {
		calls.Add(1)
		return nil, errors.New("db down")
	}
	if _, err := c.get(context.Background(), time.Minute, time.Minute, fail); err == nil {
		t.Fatal("expected error")
	}
	if _, err := c.get(context.Background(), time.Minute, time.Minute, fail); err == nil {
		t.Fatal("expected suppressed error")
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 load within the negative window, got %d", calls.Load())
	}
}
