# refresh

[![GoDoc](https://godoc.org/go.tmthrgd.dev/refresh?status.svg)](https://godoc.org/go.tmthrgd.dev/refresh)
[![Build Status](https://travis-ci.com/tmthrgd/refresh.svg?branch=master)](https://travis-ci.com/tmthrgd/refresh)

A package to call a function at most once every duration. It is loosely similar to sync.Once, but with periodic refresh.

It doesn't spawn any background goroutines and never refreshes more than is needed. The provided function is only ever called during `Load`.

## Usage

```go
import "go.tmthrgd.dev/refresh"
```

The following is an example of using **refresh** with [`net/http`](https://golang.org/pkg/net/http/). `expensiveCall` will be called no more than once every 30 minutes and only when `Load` is called.

```go
r := refresh.New(30*time.Minute, func() (interface{}, error) {
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
```

## License

[BSD 3-Clause License](LICENSE)
