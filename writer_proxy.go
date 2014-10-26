package main

// Note: taken from:
//	 https://github.com/goji/glogrus/blob/master/writer_proxy.go

import (
	"net/http"
)

// wrapWriter returns a proxy that wraps ResponseWriter
func wrapWriter(w http.ResponseWriter) writerProxy {
	bw := basicWriter{ResponseWriter: w}
	return &bw
}

// writerProxy is a proxy that wraps ResponseWriter
type writerProxy interface {
	http.ResponseWriter
	maybeWriteHeader()
	status() int
	written() uint64
}

// basicWriter holds the status code and a
// flag in addition to http.ResponseWriter
type basicWriter struct {
	http.ResponseWriter
	wroteHeader bool
	code        int
	bytes       uint64
}

// WriteHeader stores the status code and writes header
func (b *basicWriter) WriteHeader(code int) {
	if !b.wroteHeader {
		b.code = code
		b.wroteHeader = true
		b.ResponseWriter.WriteHeader(code)
	}
}

// Write writes the bytes and calls MaybeWriteHeader
func (b *basicWriter) Write(buf []byte) (int, error) {
	b.maybeWriteHeader()
	b.bytes += uint64(len(buf))
	return b.ResponseWriter.Write(buf)
}

// maybeWriteHeader writes the header if it is not alredy set
func (b *basicWriter) maybeWriteHeader() {
	if !b.wroteHeader {
		b.WriteHeader(http.StatusOK)
	}
}

// status returns the status
func (b *basicWriter) status() int {
	return b.code
}

func (b *basicWriter) written() uint64 {
	return b.bytes
}

// unwrap returns the original http.ResponseWriter
func (b *basicWriter) Unwrap() http.ResponseWriter {
	return b.ResponseWriter
}
