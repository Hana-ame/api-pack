package tools

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"

	"github.com/gen2brain/avif"
	"golang.org/x/image/webp"
)

// https://github.com/buckket/go-blurhash
// for image encode to blurhash

// DecodeResponseToImage decodes an HTTP response body into an image.Image.
// It supports JPEG, PNG, GIF, and WebP formats based on the Content-Type header.
func DecodeResponseToImage(r *http.Response) (image.Image, error) {
	// Ensure the body is closed when the function returns,
	// regardless of how it returns (normal exit, panic, error).
	defer r.Body.Close()

	contentType := r.Header.Get("Content-Type")
	switch contentType {
	// case "image/x-icon": // ico.Decode is not a standard library package, ensure you have it if uncommented
	// 	return ico.Decode(r.Body) // Example: you'd need a package like "github.com/biessek/golang-ico"
	case "image/jpeg", "image/jpg": // Added image/jpg as it's also common
		return jpeg.Decode(r.Body)
	case "image/png":
		return png.Decode(r.Body)
	case "image/gif":
		return gif.Decode(r.Body)
	case "image/webp": // Added WebP support
		return webp.Decode(r.Body)
	case "image/avif": // Added WebP support
		return avif.Decode(r.Body)
	default:
		// As an alternative or fallback, you could try image.Decode which attempts to auto-detect
		// For this to work with webp, you'd need to import it with an underscore if not used directly:
		// import _ "golang.org/x/image/webp"
		img, fn, err := image.Decode(r.Body) // The second return value is the format string
		if err != nil {
			return nil, fmt.Errorf("unsupported or malformed image format for Content-Type '%s': %w [%s]", contentType, err, fn)
		}
		return img, nil
		//
		// For now, sticking to explicit type check:
		// return nil, fmt.Errorf("unsupported image Content-Type: %s [%s]", contentType)
	}
}

// Example usage (optional, for testing)
// func main() {
// 	// This is a placeholder. In a real scenario, you'd get an *http.Response
// 	// from an http.Get or similar.
// 	// For example, to test, you could set up a mock server or fetch a real image.

// 	// Example: Fetching a known WebP image (replace with a real URL if testing)
// 	// resp, err := http.Get("https://www.gstatic.com/webp/gallery/1.webp")
// 	// if err != nil {
// 	// 	log.Fatalf("Failed to get image: %v", err)
// 	// }
// 	//
// 	// img, err := DecodeResponseToImage(resp)
// 	// if err != nil {
// 	// 	log.Fatalf("Failed to decode image: %v", err)
// 	// }
// 	//
// 	// fmt.Printf("Successfully decoded image. Dimensions: %s\n", img.Bounds())
// }

// func Read
