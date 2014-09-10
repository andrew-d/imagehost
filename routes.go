package main

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/mitchellh/goamz/s3"
)

func Index(w http.ResponseWriter, r *http.Request, p routeParams) {
	fmt.Fprint(w, "This is the main page")
}

func Upload(w http.ResponseWriter, r *http.Request, p routeParams) {
	if !doBasicAuth(w, r, p) {
		w.Header().Set("WWW-Authenticate", "Basic realm=\"user\"")
		printError(w, errorInfo{msg: "unauthorized", code: 403})
		return
	}

	// Store up to 5 MiB in memory
	err := r.ParseMultipartForm(5 * 1024 * 1024)
	if err != nil {
		printError(w, errorInfo{err: err, msg: "error parsing request form"})
		return
	}

	files, found := r.MultipartForm.File["upload"]
	if !found || len(files) < 1 {
		printError(w, errorInfo{msg: "'upload' not found", code: 400})
		return
	}
	file := files[0]

	log.Printf("Got upload with name: %s", file.Filename)

	f, err := file.Open()
	if err != nil {
		printError(w, errorInfo{msg: "error opening multipart file", err: err})
		return
	}
	defer f.Close()

	// Find the size of the file.
	size, err := getSize(f)
	if err != nil {
		printError(w, errorInfo{msg: "error finding size of file", err: err})
		return
	}

	// Try decoding the input as an image.
	contentType, ok := checkImage(f)
	if !ok {
		printError(w, errorInfo{msg: "input does not appear to be an image", code: 400})
		return
	}

	// If there's an archive bucket, save there.
	if len(p.config.ArchiveBucket) > 0 {
		b := p.s3.Bucket(p.config.ArchiveBucket)
		err = b.PutReader(file.Filename, f, size, contentType, s3.BucketOwnerFull)
		if err != nil {
			printError(w, errorInfo{msg: "error saving to archive bucket", err: err})
			return
		}

		// We need to seek back to the beginning of the file, since the above reads
		// until EOF
		_, err = f.Seek(0, 0)
		if err != nil {
			printError(w, errorInfo{msg: "error saving to archive bucket", err: err})
			return
		}
	}

	// Sanitize the image.
	// TODO: add support for animated GIFs
	sanitized, err := SanitizeImageFrom(f)
	if err != nil {
		printError(w, errorInfo{msg: "error sanitizing image", err: err})
		return
	}

	// Generate a random name for this image.
	publicName := randString(10)

	// Save to the public bucket.
	b := p.s3.Bucket(p.config.PublicBucket)
	err = b.PutReader(publicName, sanitized, size, contentType, s3.PublicRead)
	if err != nil {
		printError(w, errorInfo{msg: "error saving to public bucket", err: err})
		return
	}

	// Get the URL of the uploaded file and return it.
	publicURL := b.URL(publicName)

	w.WriteHeader(200)
	printJson(w, map[string]interface{}{
		"status":     "ok",
		"public_url": publicURL,
	})
}

func getSize(s io.Seeker) (size int64, err error) {
	if _, err = s.Seek(0, 0); err != nil {
		return
	}

	if size, err = s.Seek(2, 0); err != nil {
		return
	}

	_, err = s.Seek(0, 0)
	return
}

// If this returns 'true', then the user is authorized.
func doBasicAuth(w http.ResponseWriter, r *http.Request, p routeParams) bool {
	authorizationArray := r.Header["Authorization"]

	if len(authorizationArray) > 0 {
		authorization := strings.TrimSpace(authorizationArray[0])
		credentials := strings.Split(authorization, " ")

		if len(credentials) != 2 || credentials[0] != "Basic" {
			return false
		}

		authStr, err := base64.StdEncoding.DecodeString(credentials[1])
		if err != nil {
			return false
		}

		authParts := strings.Split(string(authStr), ":")
		if len(authParts) != 2 {
			return false
		}

		equal := 0
		equal += stringsEqSecure(authParts[0], p.config.Auth.Username)
		equal += stringsEqSecure(authParts[1], p.config.Auth.Password)
		if equal == 2 {
			return true
		} else {
			return false
		}
	} else {
		return false
	}
}

// Returns 1 if strings are equal, 0 otherwise.
func stringsEqSecure(x, y string) int {
	if len(x) != len(y) {
		return 0
	}

	b1 := []byte(x)
	b2 := []byte(y)

	return subtle.ConstantTimeCompare(b1, b2)
}
