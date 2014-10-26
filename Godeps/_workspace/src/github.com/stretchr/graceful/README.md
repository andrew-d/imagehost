graceful [![GoDoc](https://godoc.org/github.com/stretchr/graceful?status.png)](http://godoc.org/github.com/stretchr/graceful) [![wercker status](https://app.wercker.com/status/2729ba763abf87695a17547e0f7af4a4/s "wercker status")](https://app.wercker.com/project/bykey/2729ba763abf87695a17547e0f7af4a4)
========

Graceful is a Go 1.3+ package enabling graceful shutdown of http.Handler servers.

## Usage

Usage of Graceful is simple. Create your http.Handler and pass it to the `Run` function:

```go
import (
  "github.com/stretchr/graceful"
  "net/http"
  "fmt"
)

func main() {
  mux := http.NewServeMux()
  mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
    fmt.Fprintf(w, "Welcome to the home page!")
  })

  graceful.Run(":3001",10*time.Second,mux)
}
```

Another example, using [Negroni](https://github.com/codegangsta/negroni), functions in much the same manner:

```go
package main

import (
  "github.com/codegangsta/negroni"
  "github.com/stretchr/graceful"
  "net/http"
  "fmt"
)

func main() {
  mux := http.NewServeMux()
  mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
    fmt.Fprintf(w, "Welcome to the home page!")
  })

  n := negroni.Classic()
  n.UseHandler(mux)
  //n.Run(":3000")
  graceful.Run(":3001",10*time.Second,n)
}
```

In addition to Run there are the http.Server counterparts ListenAndServe, ListenAndServeTLS and Serve, which allow you to configure HTTPS, custom timeouts and error handling.
Graceful may also be used by instantiating its Server type directly, which embeds an http.Server:

```go
mux := // ...

srv := &graceful.Server{
  Timeout: 10 * time.Second,

  Server: &http.Server{
    Addr: ":1234",
    Handler: mux,
  },
}

srv.ListenAndServe()
```

This form allows you to set the ConnState callback, which works in the same way as in http.Server:

```go
mux := // ...

srv := &graceful.Server{
  Timeout: 10 * time.Second,

  ConnState: func(conn net.Conn, state http.ConnState) {
    // conn has a new state
  },

  Server: &http.Server{
    Addr: ":1234",
    Handler: mux,
  },
}

srv.ListenAndServe()
```

## Behaviour

When Graceful is sent a SIGINT or SIGTERM (possibly from ^C or a kill command), it:

1. Disables keepalive connections.
2. Closes the listening socket, allowing another process to listen on that port immediately.
3. Starts a timer of `timeout` duration to give active requests a chance to finish.
4. When timeout expires, closes all active connections.
5. Closes the `stopChan`, waking up any blocking goroutines.
6. Returns from the function, allowing the server to terminate.

## Notes

If the `timeout` argument to `Run` is 0, the server never times out, allowing all active requests to complete.

If you wish to stop the server in some way other than an OS signal, you may call the `Stop()` function.
This function stops the server, gracefully, using the new timeout value you provide. The `StopChan()` function
returns a channel on which you can block while waiting for the server to stop. This channel will be closed when
the server is stopped, allowing your execution to proceed. Multiple goroutines can block on this channel at the
same time and all will be signalled when stopping is complete.

## Contributing

Before sending a pull request, please open a new issue describing the feature/issue you wish to address so it can be discussed. The subsequent pull request should close that issue.
