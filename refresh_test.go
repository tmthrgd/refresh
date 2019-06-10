package refresh

import (
	"errors"
	"io"
	"net/http"
	"runtime"
	"sync"
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
	var called int
	r := New(time.Hour, func() (interface{}, error) {
		called++
		return called, nil
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

	assert.Equal(t, 1, called)
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
