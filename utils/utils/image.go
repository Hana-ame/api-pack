package tools

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"

	"golang.org/x/image/webp"
)

func DecodeResponseToImage(r *http.Response) (image.Image, error) {
	defer r.Body.Close()

	contentType := r.Header.Get("Content-Type")
	switch contentType {
	case "image/jpeg", "image/jpg":
		return jpeg.Decode(r.Body)
	case "image/png":
		return png.Decode(r.Body)
	case "image/gif":
		return gif.Decode(r.Body)
	case "image/webp":
		return webp.Decode(r.Body)
	case "image/avif":
		img, _, err := image.Decode(r.Body)
		return img, err
	default:
		img, fn, err := image.Decode(r.Body)
		if err != nil {
			return nil, fmt.Errorf("unsupported image format '%s': %w [%s]", contentType, err, fn)
		}
		return img, nil
	}
}
