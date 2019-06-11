// Package refresh allows calling a function at most once every duration.
// It is loosely similar to sync.Once, but with periodic refresh.
package refresh // import "go.tmthrgd.dev/refresh"

import (
	"sync"
	"sync/atomic"
	"time"
)

var testNeedsRefresh = false

type value struct {
	val interface{}
	err error

	when         time.Time
	refresh      sync.Once // used if !staleWhileRefresh || when.IsZero()
	refreshStale uint32    // used if staleWhileRefresh && !when.IsZero()
}

// Refresher is an object that will perform an action at most once every
// specified duration.
type Refresher struct {
	val atomic.Value // *value

	maxAge    time.Duration
	refreshFn func() (interface{}, error)

	staleWhileRefresh bool
}

// New returns a Refresher that will call refreshFn at most once every maxAge
// duration. refreshFn will not be called until Load is called.
//
// refreshFn will be called in the same goroutine as Load.
func New(maxAge time.Duration, refreshFn func() (interface{}, error)) *Refresher {
	if maxAge <= 0 {
		panic("refresh: maxAge must be positive duration")
	}

	r := &Refresher{atomic.Value{}, maxAge, refreshFn, false}
	r.val.Store(new(value))
	return r
}

// SetStaleWhileRefresh controls the behaviour of Load when the value is stale.
// When set to true, only one call to Load will block while any others return
// stale data. When set to false, all calls to Load will block and only ever
// return fresh data. It defaults to false.
func (r *Refresher) SetStaleWhileRefresh(v bool) {
	r.staleWhileRefresh = v
}

// Load returns a value that is at most maxAge old. Load will only ever return
// an error that was returned from refreshFn.
//
// The behaviour of Load when the value is stale can be controlled by
// SetStaleWhileRefresh. If the value is stale, it will either block all Load
// calls to call the refreshFn given to New, or only the first Load call.
func (r *Refresher) Load() (interface{}, error) {
	val := r.val.Load().(*value)
	switch {
	case testNeedsRefresh:
	case val.when.IsZero(): // first Load
	case time.Since(val.when) <= r.maxAge:
		return val.val, val.err
	case r.staleWhileRefresh:
		return r.loadStale(val)
	}

	return r.loadFresh(val)
}

func (r *Refresher) loadFresh(val *value) (interface{}, error) {
	val.refresh.Do(func() {
		newVal, err := r.refreshFn()
		r.val.Store(&value{newVal, err, time.Now(), sync.Once{}, 0})
	})

	val = r.val.Load().(*value)
	return val.val, val.err
}

func (r *Refresher) loadStale(val *value) (interface{}, error) {
	if !atomic.CompareAndSwapUint32(&val.refreshStale, 0, 1) {
		return val.val, val.err
	}

	newVal, err := r.refreshFn()
	r.val.Store(&value{newVal, err, time.Now(), sync.Once{}, 0})
	return newVal, err
}
