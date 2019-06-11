package refresh

import (
	"errors"
	"io"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func expensiveCall() ([]byte, error) { return []byte("ok"), nil }

var tmpl interface {
	Execute(io.Writer, interface{}) error
}

func ExampleNew_http() {
	r := New(30*time.Minute, func() (interface{}, error) {
		return expensiveCall()
	})
	r.SetStaleWhileRefresh(true)

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		data, err := r.Load()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmpl.Execute(w, data)
	})
}

func TestRefresher(t *testing.T) {
	defer func() { testNeedsRefresh = false }()

	var (
		called int
		retErr error
	)
	r := New(time.Hour, func() (interface{}, error) {
		called++
		return called, retErr
	})

	for n := 1; n <= 2; n++ {
		for i := 0; i < 10; i++ {
			testNeedsRefresh = n > 1 && i == 0

			c, err := r.Load()
			assert.NoError(t, err)
			assert.Equal(t, n, c)
		}
	}

	retErr = errors.New("error")

	for i := 0; i < 10; i++ {
		c, err := r.Load()
		assert.NoError(t, err)
		assert.Equal(t, 2, c)
	}

	for i := 0; i < 10; i++ {
		testNeedsRefresh = i == 0

		c, err := r.Load()
		assert.EqualError(t, err, retErr.Error())
		assert.Equal(t, 3, c)
	}

	assert.Equal(t, 3, called)
}

func TestRefresherParallel(t *testing.T) {
	var called int32
	r := New(time.Hour, func() (interface{}, error) {
		return int(atomic.AddInt32(&called, 1)), nil
	})

	var (
		wg   sync.WaitGroup
		wait = make(chan struct{})
	)
	for n := 0; n < runtime.NumCPU(); n++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			<-wait

			c, err := r.Load()
			assert.NoError(t, err)
			assert.Equal(t, 1, c)
		}()
	}

	close(wait)
	wg.Wait()

	assert.Equal(t, int32(1), called)
}

func TestRefresherTime(t *testing.T) {
	var called int
	r := New(time.Millisecond, func() (interface{}, error) {
		called++
		return called, nil
	})

	c, err := r.Load()
	assert.NoError(t, err)
	assert.Equal(t, 1, c)

	time.Sleep(10 * time.Millisecond)

	c, err = r.Load()
	assert.NoError(t, err)
	assert.Equal(t, 2, c)
}

func TestRefresherStaleParallel(t *testing.T) {
	var called int32
	r := New(time.Millisecond, func() (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return int(atomic.AddInt32(&called, 1)), nil
	})
	r.SetStaleWhileRefresh(true)

	// Run the first Load and wait for the data to be stale.
	c, err := r.Load()
	assert.NoError(t, err)
	assert.Equal(t, 1, c)

	time.Sleep(10 * time.Millisecond)

	var (
		wg       sync.WaitGroup
		wait     = make(chan struct{})
		sawFresh int32
	)
	for n := 0; n < runtime.NumCPU(); n++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			<-wait

			c, err := r.Load()
			assert.NoError(t, err)
			if assert.Truef(t, c == 1 || c == 2, "c(%d) should be 1 or 2", c) {
				atomic.AddInt32(&sawFresh, int32(c.(int))-1)
			}
		}()
	}

	close(wait)
	wg.Wait()

	assert.Equal(t, int32(2), called)
	assert.Equal(t, int32(1), sawFresh, "sawFresh") // The rest must have seen stale data.
}

func TestRefresherStaleParallelFirstLoad(t *testing.T) {
	var called int32
	r := New(time.Hour, func() (interface{}, error) {
		return int(atomic.AddInt32(&called, 1)), nil
	})
	r.SetStaleWhileRefresh(true)

	var (
		wg   sync.WaitGroup
		wait = make(chan struct{})
	)
	for n := 0; n < runtime.NumCPU(); n++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			<-wait

			c, err := r.Load()
			assert.NoError(t, err)
			assert.Equal(t, 1, c)
		}()
	}

	close(wait)
	wg.Wait()

	assert.Equal(t, int32(1), called)
}

func TestNewPanicsForInvalidMaxAge(t *testing.T) {
	dummy := func() (interface{}, error) { return nil, nil }
	assert.PanicsWithValue(t, "refresh: maxAge must be positive duration", func() {
		New(0, dummy)
	}, "maxAge is 0")
	assert.PanicsWithValue(t, "refresh: maxAge must be positive duration", func() {
		New(-1, dummy)
	}, "maxAge is -1")
}
