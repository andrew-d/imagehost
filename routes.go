package main

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/mitchellh/goamz/s3"
	"github.com/zenazn/goji/web"
)

func Index(c web.C, w http.ResponseWriter, r *http.Request) {
	config := c.Env["config"].(*Config)

	renderTemplate(w, "index", M{
		"baseUrl": config.BaseURL,
	})
}

func Upload(c web.C, w http.ResponseWriter, r *http.Request) {
	client := c.Env["client"].(*s3.S3)
	config := c.Env["config"].(*Config)

	// Store up to 5 MiB in memory
	err := r.ParseMultipartForm(5 * 1024 * 1024)
	if err != nil {
		renderError(w, http.StatusBadRequest, err.Error(), "error parsing request form")
		return
	}

	f, filename, size, err := extractFile(r, "upload")
	if err != nil {
		renderError(w, http.StatusBadRequest, err.Error(), "error extracting uploaded file")
		return
	}
	defer f.Close()

	// Try decoding the input as an image.
	imageFormat, ok := checkImage(f)
	if !ok {
		renderError(w, http.StatusBadRequest, "not an image", "input does not appear to be an image")
		return
	}
	contentType := "image/" + imageFormat

	log.WithFields(logrus.Fields{
		"name":   filename,
		"size":   size,
		"format": imageFormat,
	}).Info("got upload")

	// If there's an archive bucket, save there.
	if len(config.ArchiveBucket) > 0 {
		b := client.Bucket(config.ArchiveBucket)
		err = b.PutReader(filename, f, size, contentType, s3.BucketOwnerFull)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err.Error(), "error saving to archive bucket")
			return
		}

		log.WithFields(logrus.Fields{
			"name":        filename,
			"archive_url": b.URL(filename),
		}).Info("uploaded archive image")

		// We need to seek back to the beginning of the file, since the above reads
		// until EOF
		_, err = f.Seek(0, 0)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err.Error(), "error saving to archive bucket")
			return
		}
	}

	// Sanitize the image.
	// TODO: add support for animated GIFs
	sanitized, size, err := SanitizeImageFrom(f, config.JPEGCompression)
	if err != nil {
		renderError(w, http.StatusInternalServerError, err.Error(), "error sanitizing image")
		return
	}

	// Generate a random name for this image.
	publicName := randString(10) + "." + imageFormat

	log.WithFields(logrus.Fields{
		"name":           filename,
		"sanitized_size": size,
		"public_name":    publicName,
	}).Info("image sanitized")

	// Save to the public bucket.
	b := client.Bucket(config.PublicBucket)
	err = b.PutReader(publicName, sanitized, size, contentType, s3.PublicRead)
	if err != nil {
		renderError(w, http.StatusInternalServerError, err.Error(), "error saving to public bucket")
		return
	}

	// Get the URL of the uploaded file and return it.
	publicURL := b.URL(publicName)

	log.WithFields(logrus.Fields{
		"name":       filename,
		"public_url": publicURL,
	}).Info("uploaded public image")

	renderJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "ok",
		"public_url": publicURL,
	})
}

// Extracts a file from a HTTP request.  Returns the file and its size.
func extractFile(r *http.Request, name string) (multipart.File, string, int64, error) {
	files, found := r.MultipartForm.File[name]
	if !found || len(files) < 1 {
		return nil, "", 0, fmt.Errorf("'%s' not found", name)
	}
	file := files[0]

	f, err := file.Open()
	if err != nil {
		return nil, "", 0, errors.New("could not open multipart file")
	}

	// Find the size of the file.
	size, err := getSize(f)
	if err != nil {
		return nil, "", 0, errors.New("could not find size of file")
	}

	return f, file.Filename, size, nil
}
