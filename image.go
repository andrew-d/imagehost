package main

import (
	"fmt"
	"bytes"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
)

func checkImage(r io.ReadSeeker) (string, bool) {
	_, fmt, err := image.Decode(r)
	_, err2 := r.Seek(0, 0)
	if err != nil || err2 != nil {
		return "", false
	}

	return "image/" + fmt, true
}

func SanitizeImageFrom(r io.Reader) (io.ReadSeeker, error) {
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	log.Printf("Sanitizing image of format: %s", format)
	newImg := CloneToRGBA(img)

	// Encode as the original type into a buffer
	var buf bytes.Buffer
	switch format {
	case "gif":
		err = gif.Encode(&buf, newImg, &gif.Options{NumColors: 256})
	case "jpeg":
		err = jpeg.Encode(&buf, newImg, &jpeg.Options{Quality: 80})
	case "png":
		err = png.Encode(&buf, newImg)
	default:
		return nil, fmt.Errorf("unknown image format: %s", format)
	}

	if err != nil {
		return nil, err
	}

	// Convert to a byte slice, and then to our ReadSeeker.
	bSlice := buf.Bytes()
	return bytes.NewReader(bSlice), nil
}

func CloneToRGBA(src image.Image) draw.Image {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, src, b.Min, draw.Src)
	return dst
}
