// gemini

package tools

import (
	"fmt"
	"image"
	"log"
	"net/http"
	"os"
	"testing"

	"golang.org/x/image/webp"
)

func TestRespToImg(t *testing.T) {
	// This is a placeholder. In a real scenario, you'd get an *http.Response
	// from an http.Get or similar.
	// For example, to test, you could set up a mock server or fetch a real image.

	// Example: Fetching a known WebP image (replace with a real URL if testing)
	resp, err := http.Get("https://upload.moonchan.xyz/api/01LLWEUU2LK7VKGI67L5GKX5ZBW7OKKQYZ/output_001.webp")
	if err != nil {
		log.Fatalf("Failed to get image: %v", err)
	}
	defer resp.Body.Close()

	img, err := webp.Decode(resp.Body)
	if err != nil {
		log.Fatalf("Failed to decode image: %v", err)
	}

	fmt.Printf("Successfully decoded image. Dimensions: %s\n", img.Bounds())
}

func TestWebp(t *testing.T) {
	webpFile, err := os.Open("/mnt/c//Users/lumin/Downloads/output_001.webp")
	if err != nil {
		log.Fatalf("err: %v", err)
	}
	img, err := webp.Decode(webpFile)
	if err != nil {
		log.Fatalf("err: %v", err)
	}
	fmt.Printf("Successfully decoded image. Dimensions: %s\n", img.Bounds())
	img, fn, err := image.Decode(webpFile) // The second return value is the format string
	if err != nil {
		log.Fatalf("unsupported or malformed image format for Content-Type '%s': [%s]", err, fn)
	}
	fmt.Printf("Successfully decoded image. Dimensions: %s\n", fn)
	fmt.Printf("Successfully decoded image. Dimensions: %s\n", img.Bounds())
}
