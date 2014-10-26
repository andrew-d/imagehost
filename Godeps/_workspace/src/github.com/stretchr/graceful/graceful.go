package graceful

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/stretchr/pat/stop"
	"code.google.com/p/go.net/netutil"
)

// Server wraps an http.Server with graceful connection handling.
// It may be used directly in the same way as http.Server, or may
// be constructed with the global functions in this package.
//
// Example:
//	srv := &graceful.Server{
//		Timeout: 5 * time.Second,
//		Server: &http.Server{Addr: ":1234", Handler: handler},
//	}
//	srv.ListenAndServe()
type Server struct {
	*http.Server

	// Timeout is the duration to allow outstanding requests to survive
	// before forcefully terminating them.
	Timeout time.Duration

	// Limit the number of outstanding requests
	ListenLimit int

	// ConnState specifies an optional callback function that is
	// called when a client connection changes state. This is a proxy
	// to the underlying http.Server's ConnState, and the original
	// must not be set directly.
	ConnState func(net.Conn, http.ConnState)

	// ShutdownInitiated is an optional  callback function that is called
	// when shutdown is initiated. It can be used to notify the client
	// side of long lived connections (e.g. websockets) to reconnect.
	ShutdownInitiated func()

	// interrupt signals the listener to stop serving connections,
	// and the server to shut down.
	interrupt chan os.Signal

	// stopChan is the channel on which callers may block while waiting for
	// the server to stop.
	stopChan chan stop.Signal

	// stopChanOnce is used to create the stop channel on demand, once, per
	// instance.
	stopChanOnce sync.Once

	// connections holds all connections managed by graceful
	connections map[net.Conn]struct{}
}

// ensure Server conforms to stop.Stopper
var _ stop.Stopper = (*Server)(nil)

// Run serves the http.Handler with graceful shutdown enabled.
//
// timeout is the duration to wait until killing active requests and stopping the server.
// If timeout is 0, the server never times out. It waits for all active requests to finish.
func Run(addr string, timeout time.Duration, n http.Handler) {
	srv := &Server{
		Timeout: timeout,
		Server:  &http.Server{Addr: addr, Handler: n},
	}

	if err := srv.ListenAndServe(); err != nil {
		if opErr, ok := err.(*net.OpError); !ok || (ok && opErr.Op != "accept") {
			logger := log.New(os.Stdout, "[graceful] ", 0)
			logger.Fatal(err)
		}
	}
}

// ListenAndServe is equivalent to http.Server.ListenAndServe with graceful shutdown enabled.
//
// timeout is the duration to wait until killing active requests and stopping the server.
// If timeout is 0, the server never times out. It waits for all active requests to finish.
func ListenAndServe(server *http.Server, timeout time.Duration) error {
	srv := &Server{Timeout: timeout, Server: server}
	return srv.ListenAndServe()
}

// ListenAndServe is equivalent to http.Server.ListenAndServe with graceful shutdown enabled.
func (srv *Server) ListenAndServe() error {
	// Create the listener so we can control their lifetime
	addr := srv.Addr
	if addr == "" {
		addr = ":http"
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	if srv.ListenLimit != 0 {
		l = netutil.LimitListener(l, srv.ListenLimit)
	}
	return srv.Serve(l)
}

// ListenAndServeTLS is equivalent to http.Server.ListenAndServeTLS with graceful shutdown enabled.
//
// timeout is the duration to wait until killing active requests and stopping the server.
// If timeout is 0, the server never times out. It waits for all active requests to finish.
func ListenAndServeTLS(server *http.Server, certFile, keyFile string, timeout time.Duration) error {
	// Create the listener ourselves so we can control its lifetime
	srv := &Server{Timeout: timeout, Server: server}
	addr := srv.Addr
	if addr == "" {
		addr = ":https"
	}

	config := &tls.Config{}
	if srv.TLSConfig != nil {
		*config = *srv.TLSConfig
	}
	if config.NextProtos == nil {
		config.NextProtos = []string{"http/1.1"}
	}

	var err error
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	conn, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	tlsListener := tls.NewListener(conn, config)
	return srv.Serve(tlsListener)
}

// Serve is equivalent to http.Server.Serve with graceful shutdown enabled.
//
// timeout is the duration to wait until killing active requests and stopping the server.
// If timeout is 0, the server never times out. It waits for all active requests to finish.
func Serve(server *http.Server, l net.Listener, timeout time.Duration) error {
	srv := &Server{Timeout: timeout, Server: server}
	return srv.Serve(l)
}

// Serve is equivalent to http.Server.Serve with graceful shutdown enabled.
func (srv *Server) Serve(listener net.Listener) error {
	// Track connection state
	add := make(chan net.Conn)
	remove := make(chan net.Conn)

	srv.Server.ConnState = func(conn net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			add <- conn
		case http.StateClosed, http.StateHijacked:
			remove <- conn
		}

		if srv.ConnState != nil {
			srv.ConnState(conn, state)
		}
	}

	// Manage open connections
	shutdown := make(chan chan struct{})
	kill := make(chan struct{})
	go func() {
		var done chan struct{}
		srv.connections = map[net.Conn]struct{}{}
		for {
			select {
			case conn := <-add:
				srv.connections[conn] = struct{}{}
			case conn := <-remove:
				delete(srv.connections, conn)
				if done != nil && len(srv.connections) == 0 {
					done <- struct{}{}
					return
				}
			case done = <-shutdown:
				if len(srv.connections) == 0 {
					done <- struct{}{}
					return
				}
			case <-kill:
				for k := range srv.connections {
					k.Close()
				}
				return
			}
		}
	}()

	if srv.interrupt == nil {
		srv.interrupt = make(chan os.Signal, 1)
	}

	// Set up the interrupt catch
	signal.Notify(srv.interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-srv.interrupt
		srv.SetKeepAlivesEnabled(false)
		listener.Close()

		if srv.ShutdownInitiated != nil {
			srv.ShutdownInitiated()
		}

		signal.Stop(srv.interrupt)
		close(srv.interrupt)
	}()

	// Serve with graceful listener.
	// Execution blocks here until listener.Close() is called, above.
	err := srv.Server.Serve(listener)

	// Request done notification
	done := make(chan struct{})
	shutdown <- done

	if srv.Timeout > 0 {
		select {
		case <-done:
		case <-time.After(srv.Timeout):
			close(kill)
		}
	} else {
		<-done
	}
	// Close the stopChan to wake up any blocked goroutines.
	if srv.stopChan != nil {
		close(srv.stopChan)
	}
	return err
}

// Stop instructs the type to halt operations and close
// the stop channel when it is finished.
//
// timeout is grace period for which to wait before shutting
// down the server. The timeout value passed here will override the
// timeout given when constructing the server, as this is an explicit
// command to stop the server.
func (srv *Server) Stop(timeout time.Duration) {
	srv.Timeout = timeout
	srv.interrupt <- syscall.SIGINT
}

// StopChan gets the stop channel which will block until
// stopping has completed, at which point it is closed.
// Callers should never close the stop channel.
func (srv *Server) StopChan() <-chan stop.Signal {
	srv.stopChanOnce.Do(func() {
		if srv.stopChan == nil {
			srv.stopChan = stop.Make()
		}
	})
	return srv.stopChan
}
