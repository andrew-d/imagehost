package main

import (
	"errors"
	"io"
	"log"

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

	files, found := c.Request.MultipartForm.File["upload"]
	if !found || len(files) < 1 {
		c.Error(err, "'upload' not found")
		c.Abort(400)
		return
	}
	file := files[0]

	log.Printf("Got upload with name: %s", file.Filename)

	f, err := file.Open()
	if err != nil {
		c.Error(err, "error opening multipart file")
		return
	}
	defer f.Close()

	// Find the size of the file.
	size, err := getSize(f)
	if err != nil {
		c.Error(err, "error finding size of file")
		return
	}

	log.Printf("size of file: %d", size)

	// Try decoding the input as an image.
	contentType, ok := checkImage(f)
	if !ok {
		c.Error(errors.New("not an image"), "input does not appear to be an image")
		c.Abort(400)
		return
	}

	// If there's an archive bucket, save there.
	if len(config.ArchiveBucket) > 0 {
		b := client.Bucket(config.ArchiveBucket)
		err = b.PutReader(file.Filename, f, size, contentType, s3.BucketOwnerFull)
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
	publicName := randString(10)

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
