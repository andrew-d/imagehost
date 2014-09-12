package main

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/goamz/s3"
)

func Index(c *gin.Context) {
	c.String(200, "This is the main page")
}

func Upload(c *gin.Context) {
	client := c.MustGet("client").(*s3.S3)
	config := c.MustGet("config").(*Config)

	// Store up to 5 MiB in memory
	err := c.Request.ParseMultipartForm(5 * 1024 * 1024)
	if err != nil {
		c.Error(err, "error parsing request form")
		c.Abort(400)
		return
	}

	f, filename, size, err := extractFile(c.Request, "upload")
	if err != nil {
		c.Error(err, "error extracting uploaded file")
		c.Abort(400)
		return
	}
	defer f.Close()

	// Try decoding the input as an image.
	imageFormat, ok := checkImage(f)
	if !ok {
		c.Error(errors.New("not an image"), "input does not appear to be an image")
		c.Abort(400)
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
			c.Error(err, "error saving to archive bucket")
			return
		}

		// We need to seek back to the beginning of the file, since the above reads
		// until EOF
		_, err = f.Seek(0, 0)
		if err != nil {
			c.Error(err, "error saving to archive bucket")
			return
		}
	}

	// Sanitize the image.
	// TODO: add support for animated GIFs
	sanitized, size, err := SanitizeImageFrom(f)
	if err != nil {
		c.Error(err, "error sanitizing image")
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
		c.Error(err, "error saving to public bucket")
		return
	}

	// Get the URL of the uploaded file and return it.
	publicURL := b.URL(publicName)

	c.JSON(200, map[string]interface{}{
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
