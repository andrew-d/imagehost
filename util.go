package main

import (
	"crypto/rand"
	"io"
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
