package main

import (
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path"
	"testing"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/stretchr/testify/assert"
)

func TestImageOrientation(t *testing.T) {
	for i := 1; i <= 8; i++ {
		fname := fmt.Sprintf("Landscape_%d.jpg", i)
		f, err := os.Open(path.Join("exif-orientation-examples", fname))
		assert.NoError(t, err)
		defer f.Close()

		img, _, err := image.Decode(f)
		assert.NoError(t, err)

		newImg := CloneToRGBA(img)

		_, err = f.Seek(0, 0)
		assert.NoError(t, err)

		ex, err := parseExif(f)
		assert.NoError(t, err)

		orientation, err := ex.Get(exif.Orientation)
		assert.NoError(t, err)

		newImg, err = fixOrientation(newImg, orientation, ex.Tiff.Order)
		assert.NoError(t, err)

		outFile, err := os.Create(path.Join("exif-orientation-processed", "processed-" + fname))
		assert.NoError(t, err)

		err = jpeg.Encode(outFile, newImg, &jpeg.Options{Quality: 100})
		assert.NoError(t, err)

		_ = newImg
	}
}
