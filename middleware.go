package main

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/zenazn/goji/web"
	"github.com/zenazn/goji/web/middleware"
)

var (
	log = logrus.New()
)

func DurationToString(d time.Duration) string {
	duration := d.Nanoseconds()

	var units string
	switch {
	case d > 1*1000*1000*1000:
		units = "s"
		duration /= (1000 * 1000 * 1000)

	case d > 2*1000*1000:
		// Note: we picked 2 here so we get more granularity in the
		// microsecond range
		units = "ms"
		duration /= (1000 * 1000)

	case d > 1*1000:
		units = "μs"
		duration /= 1000

	default:
		units = "ns"
	}

	return fmt.Sprintf("%d%s", duration, units)
}

func logMiddleware(c *web.C, h http.Handler) http.Handler {
	ret := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.WithFields(logrus.Fields{
			"id":     middleware.GetReqID(*c),
			"uri":    r.RequestURI,
			"method": r.Method,
			"remote": r.RemoteAddr,
		}).Info("request started")

		wrapped := wrapWriter(w)

		start := time.Now()
		h.ServeHTTP(wrapped, r)
		wrapped.maybeWriteHeader()
		duration := time.Now().Sub(start)

		log.WithFields(logrus.Fields{
			"id":       middleware.GetReqID(*c),
			"uri":      r.RequestURI,
			"method":   r.Method,
			"remote":   r.RemoteAddr,
			"status":   wrapped.status(),
			"duration": duration.Nanoseconds(),
			"size":     wrapped.written(),
		}).Infof("request finished in %s", DurationToString(duration))
	})
	return ret
}

func recoverMiddleware(c *web.C, h http.Handler) http.Handler {
	ret := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				buf := make([]byte, 1<<16)
				amt := runtime.Stack(buf, false)
				stack := string(buf[:amt])

				log.WithFields(logrus.Fields{
					"err":   err,
					"id":    middleware.GetReqID(*c),
					"stack": stack,
				}).Error("recovered from panic")

				http.Error(w, http.StatusText(500), 500)
			}
		}()

		h.ServeHTTP(w, r)
	})
	return ret
}
