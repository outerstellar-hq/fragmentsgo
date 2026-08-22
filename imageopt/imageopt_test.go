package imageopt

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// gradient renders a noisy gradient image so JPEG compression has real
// work to do — flat images would compress to almost nothing.
func gradient(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * 255) / width),
				G: uint8((y * 255) / height),
				B: uint8((x * y) % 256),
				A: 255,
			})
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image, quality int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, img, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestOptimizeJPEG(t *testing.T) {
	source := encodeJPEG(t, gradient(3200, 2000), 100)
	result, err := Optimize(source, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Format != FormatJPEG {
		t.Fatalf("format = %s", result.Format)
	}
	// Longest edge capped at the default 1600, aspect preserved.
	if result.Width != 1600 || result.Height != 1000 {
		t.Fatalf("size = %dx%d, want 1600x1000", result.Width, result.Height)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatalf("output does not decode: %v", err)
	}
	if decoded.Bounds().Dx() != 1600 {
		t.Fatalf("decoded width = %d", decoded.Bounds().Dx())
	}
	if len(result.Data) >= len(source) {
		t.Fatalf("no savings: %d -> %d bytes", len(source), len(result.Data))
	}
}

func TestOptimizePNGLossless(t *testing.T) {
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, gradient(2400, 1200)); err != nil {
		t.Fatal(err)
	}
	source := buffer.Bytes()
	result, err := Optimize(source, Options{MaxWidth: 1200})
	if err != nil {
		t.Fatal(err)
	}
	if result.Format != FormatPNG {
		t.Fatalf("format = %s", result.Format)
	}
	if result.Width != 1200 || result.Height != 600 {
		t.Fatalf("size = %dx%d, want 1200x600", result.Width, result.Height)
	}
	if _, err := png.Decode(bytes.NewReader(result.Data)); err != nil {
		t.Fatalf("output does not decode: %v", err)
	}
}

func TestOptimizePortraitAspect(t *testing.T) {
	source := encodeJPEG(t, gradient(1200, 3000), 95)
	result, err := Optimize(source, Options{MaxWidth: 800})
	if err != nil {
		t.Fatal(err)
	}
	// Longest edge is the height for portraits.
	if result.Height != 800 || result.Width != 320 {
		t.Fatalf("size = %dx%d, want 320x800", result.Width, result.Height)
	}
}

func TestOptimizePassesThroughUnknown(t *testing.T) {
	gif := append([]byte("GIF89a"), make([]byte, 64)...)
	result, err := Optimize(gif, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Format != FormatOriginal || !bytes.Equal(result.Data, gif) {
		t.Fatalf("unknown format was not passed through: %+v", result)
	}
	if _, err := Optimize(nil, Options{}); err == nil {
		t.Fatal("empty input should error")
	}
}

func TestOptimizeWithinLimitUnchangedDimensions(t *testing.T) {
	source := encodeJPEG(t, gradient(800, 600), 95)
	result, err := Optimize(source, Options{MaxWidth: 1600, JPEGQuality: 80})
	if err != nil {
		t.Fatal(err)
	}
	if result.Width != 800 || result.Height != 600 {
		t.Fatalf("small image was resized: %dx%d", result.Width, result.Height)
	}
}
