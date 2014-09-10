package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/mitchellh/goamz/s3"
)

func Index(w http.ResponseWriter, r *http.Request, p routeParams) {
	fmt.Fprint(w, "This is the main page")
}

func Upload(w http.ResponseWriter, r *http.Request, p routeParams) {
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
