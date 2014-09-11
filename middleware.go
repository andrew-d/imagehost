package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
)

var (
	idPrefix string
	reqId    uint64

	log = logrus.New()
)

func init() {
	// Init. request ID stuff.
	hostname, err := os.Hostname()
	if hostname == "" || err != nil {
		hostname = "localhost"
	}

	var buf [12]byte
	var b64 string
	for len(b64) < 10 {
		rand.Read(buf[:])
		b64 = base64.StdEncoding.EncodeToString(buf[:])
		b64 = strings.NewReplacer("+", "", "/", "").Replace(b64)
	}

	idPrefix = fmt.Sprintf("%s/%s", hostname, b64[0:10])
}

// Generate a unique request ID for each request.  Borrowed liberally from Goji.
func RequestIdMiddleware(c *gin.Context) {
	myId := atomic.AddUint64(&reqId, 1)
	c.Set("requestId", fmt.Sprintf("%s-%06d", idPrefix, myId))
	c.Next()
}

func ErrorPrintMiddleware(c *gin.Context) {
	c.Next()

	// Exit if there's no errors.
	if len(c.Errors) == 0 {
		return
	}

	type ErrorDesc struct {
		Error string      `json:"error,omitempty"`
		Meta  interface{} `json:"message,omitempty"`
	}

	errors := []ErrorDesc{}
	for _, err := range c.Errors {
		errors = append(errors, ErrorDesc{
			Error: err.Err,
			Meta:  err.Meta,
		})
	}

	resp := map[string]interface{}{
		"status": "error",
		"errors": errors,
	}

	status := c.Writer.Status()
	if status == 0 {
		status = 500
	}

	c.JSON(status, resp)
}

func LogrusMiddleware(c *gin.Context) {
	start := time.Now()
	id := c.MustGet("requestId").(string)

	log.WithFields(logrus.Fields{
		"requestId": id,
		"uri":       c.Request.RequestURI,
		"method":    c.Request.Method,
		"remote":    c.Request.RemoteAddr,
	}).Info("request_start")

	c.Next()

	latency := float64(time.Since(start)) / float64(time.Millisecond)

	log.WithFields(logrus.Fields{
		"requestId": id,
		"uri":       c.Request.RequestURI,
		"method":    c.Request.Method,
		"remote":    c.Request.RemoteAddr,
		"status":    c.Writer.Status(),
		"latency":   latency,
	}).Info("request_finished")
}
