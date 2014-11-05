package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/Sirupsen/logrus"
	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
)

func checkImage(r io.ReadSeeker) (string, bool) {
	_, fmt, err := image.Decode(r)
	_, err2 := r.Seek(0, 0)
	if err != nil || err2 != nil {
		return "", false
	}

	return fmt, true
}

func SanitizeImageFrom(r io.ReadSeeker) (io.ReadSeeker, int64, error) {
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, 0, err
	}

	var orientation *tiff.Tag
	var ex *exif.Exif

	_, err = r.Seek(0, 0)
	if err != nil {
		log.WithField("err", err).Error("Cannot rewind image to parse EXIF")
	} else if format == "jpeg" {
		ex, err = parseExif(r)

		canContinue := true
		if err != nil {
			if exif.IsCriticalError(err) {
				log.WithFields(logrus.Fields{
					"error": err,
				}).Error("Could not parse EXIF data from image")
				canContinue = false
			} else {
				log.WithFields(logrus.Fields{
					"error": err,
				}).Warn("Non-fatal error when parsing EXIF data")
			}
		}

		if canContinue {
			orientation, err = ex.Get(exif.Orientation)
			if err != nil && !exif.IsTagNotPresentError(err) {
				log.WithFields(logrus.Fields{
					"error": err,
				}).Warn("Could not get Orientation tag")
			}
		}
	}

	log.WithFields(logrus.Fields{
		"format": format,
	}).Debug("Sanitizing image")
	newImg := CloneToRGBA(img)

	if orientation != nil {
		newImg, err = fixOrientation(newImg, orientation, ex.Tiff.Order)
	}

	// Encode as the original type into a buffer
	var buf bytes.Buffer
	switch format {
	case "gif":
		err = gif.Encode(&buf, newImg, &gif.Options{NumColors: 256})
	case "jpeg":
		err = jpeg.Encode(&buf, newImg, &jpeg.Options{Quality: 100})
	case "png":
		err = png.Encode(&buf, newImg)
	default:
		return nil, 0, fmt.Errorf("unknown image format: %s", format)
	}

	if err != nil {
		return nil, 0, err
	}

	// Convert to a byte slice, and then to our ReadSeeker.
	bSlice := buf.Bytes()
	return bytes.NewReader(bSlice), int64(len(bSlice)), nil
}

func CloneToRGBA(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, src, b.Min, draw.Src)
	return dst
}

func parseExif(r io.ReadSeeker) (*exif.Exif, error) {
	ex, err := exif.Decode(r)
	_, err2 := r.Seek(0, 0)

	if err != nil {
		return nil, err
	} else if err2 != nil {
		return nil, err2
	}

	return ex, nil
}

func fixOrientation(img image.Image, orientation *tiff.Tag, order binary.ByteOrder) (image.Image, error) {
	if orientation.Type != tiff.DTShort {
		return nil, fmt.Errorf("expected orientation type to be Short, got: %d",
			orientation.Type)
	}

	if orientation.Count < 1 {
		return nil, fmt.Errorf("expected orientation tag to have values")
	}

	var orVal uint16
	r := bytes.NewReader(orientation.Val)
	err := binary.Read(r, order, &orVal)
	if err != nil {
		return nil, err
	}

	log.WithFields(logrus.Fields{
		"orientation": orVal,
	}).Info("got orientation to apply")

	var newImg image.Image

	// A diagram of the letter 'F' as if it were rotated correctly.
	// Taken from: http://sylvana.net/jpegcrop/exif_orientation.html
	//
	//   1        2       3      4         5            6           7          8
	//
	// 888888  888888      88  88      8888888888  88                  88  8888888888
	// 88          88      88  88      88  88      88  88          88  88      88  88
	// 8888      8888    8888  8888    88          8888888888  8888888888          88
	// 88          88      88  88
	// 88          88  888888  888888

	switch orVal {
	case 1:
		newImg = img
	case 2:
		newImg = imaging.FlipH(img)
	case 3:
		newImg = imaging.Rotate180(img)
	case 4:
		newImg = imaging.FlipH(imaging.Rotate180(img))
	case 5:
		newImg = imaging.FlipH(imaging.Rotate270(img))
		// newImg = imaging.Rotate270(img)
	case 6:
		newImg = imaging.Rotate270(img)
	case 7:
		newImg = imaging.FlipH(imaging.Rotate90(img))
		// newImg = imaging.Rotate90(img)
	case 8:
		newImg = imaging.Rotate90(img)

	default:
		return nil, fmt.Errorf("unknown orientation value: %d", orVal)
	}

	return newImg, nil
}
