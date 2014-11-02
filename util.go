package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"

	// We depend on this as a dummy, so Godep will vendor the go-bindata source.
	_ "github.com/jteeuwen/go-bindata"
)

// Note: slightly biased towards first 8 characters of the alphabet, since 255
// isn't a multiple of 62 (length of alphanum).  We don't really care that
// much, though.
func randString(n int) string {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, n)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}
	return string(bytes)
}

// Get the size of something that can be Seek()'d.  Resets the position back to
// the start of the Seeker afterwords.
func getSize(s io.Seeker) (size int64, err error) {
	if _, err = s.Seek(0, 0); err != nil {
		return
	}

	// 2 == from the end of the file
	if size, err = s.Seek(0, 2); err != nil {
		return
	}

	_, err = s.Seek(0, 0)
	return
}

func ServeAsset(name, mime string) http.Handler {
	// Assert that the asset exists.
	_, err := Asset(name)
	if err != nil {
		panic(fmt.Sprintf("asset named '%s' does not exist", name))
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		asset, _ := Asset(name)
		w.Header().Set("Content-Type", mime)
		w.Write(asset)
	}

	return http.HandlerFunc(handler)
}
