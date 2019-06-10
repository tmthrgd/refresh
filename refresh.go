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

	when    time.Time
	refresh sync.Once
}

// Refresher is an object that will perform an action at most once every
// specified duration.
type Refresher struct {
	val atomic.Value // *value

	maxAge    time.Duration
	refreshFn func() (interface{}, error)
}

// New returns a Refresher that will call refreshFn at most once every maxAge
// duration. refreshFn will not be called until Load is called.
//
// refreshFn will be called in the same goroutine as Load.
func New(maxAge time.Duration, refreshFn func() (interface{}, error)) *Refresher {
	if maxAge <= 0 {
		panic("refresh: maxAge must be positive duration")
	}

	r := &Refresher{atomic.Value{}, maxAge, refreshFn}
	r.val.Store(new(value))
	return r
}

// Load returns a value that is at most maxAge old. If the value is stale, it
// will block calling the refreshFn given to New. Load will only ever return an
// error that was returned from refreshFn.
func (r *Refresher) Load() (interface{}, error) {
	val := r.val.Load().(*value)
	if time.Since(val.when) <= r.maxAge && !testNeedsRefresh {
		return val.val, val.err
	}

	val.refresh.Do(func() {
		val, err := r.refreshFn()
		r.val.Store(&value{val, err, time.Now(), sync.Once{}})
	})

	val = r.val.Load().(*value)
	return val.val, val.err
}
