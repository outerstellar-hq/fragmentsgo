// Package imageopt re-encodes content images to web-friendly sizes: JPEGs
// are downscaled and re-compressed, PNGs are downscaled losslessly, and
// anything else passes through untouched. Built on the standard library
// plus golang.org/x/image — no cgo, no native codecs.
package imageopt

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"golang.org/x/image/draw"
)

// Options tune optimization.
type Options struct {
	// MaxWidth caps the image's longest edge; 0 means 1600 pixels.
	MaxWidth int
	// JPEGQuality is the re-encoding quality for JPEGs; 0 means 80.
	JPEGQuality int
}

// Result formats reported by Optimize.
const (
	FormatJPEG     = "jpeg"
	FormatPNG      = "png"
	FormatOriginal = "original"
)

// Result is the optimized image.
type Result struct {
	Data   []byte
	Format string
	Width  int
	Height int
}

func (o Options) maxWidth() int {
	if o.MaxWidth <= 0 {
		return 1600
	}
	return o.MaxWidth
}

func (o Options) jpegQuality() int {
	if o.JPEGQuality <= 0 || o.JPEGQuality > 100 {
		return 80
	}
	return o.JPEGQuality
}

// Optimize re-encodes data. JPEG inputs come back as downscaled JPEGs,
// PNG inputs as downscaled PNGs, and unknown formats (animated GIFs, WebP
// sources, SVGs) come back unchanged with FormatOriginal so callers can
// copy them on.
func Optimize(data []byte, options Options) (Result, error) {
	if len(data) == 0 {
		return Result{}, fmt.Errorf("imageopt: empty input")
	}
	kind := detect(data)
	if kind == "" {
		return Result{Data: data, Format: FormatOriginal}, nil
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		// Decodable signature but corrupt body: pass through untouched
		// rather than destroying the only copy.
		return Result{Data: data, Format: FormatOriginal}, nil
	}
	bounds := decoded.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	longest := width
	if height > longest {
		longest = height
	}
	if longest > options.maxWidth() {
		decoded = resize(decoded, options.maxWidth())
		bounds = decoded.Bounds()
		width, height = bounds.Dx(), bounds.Dy()
	}

	var buffer bytes.Buffer
	if kind == "png" {
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encoder.Encode(&buffer, decoded); err != nil {
			return Result{}, fmt.Errorf("imageopt: encode png: %w", err)
		}
		return Result{Data: buffer.Bytes(), Format: FormatPNG, Width: width, Height: height}, nil
	}
	if err := jpeg.Encode(&buffer, decoded, &jpeg.Options{Quality: options.jpegQuality()}); err != nil {
		return Result{}, fmt.Errorf("imageopt: encode jpeg: %w", err)
	}
	return Result{Data: buffer.Bytes(), Format: FormatJPEG, Width: width, Height: height}, nil
}

// detect returns "png" or "jpeg" for supported magic signatures.
func detect(data []byte) string {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return "png"
	}
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
		return "jpeg"
	}
	return ""
}

// resize scales the image so its longest edge equals limit, preserving
// aspect ratio (images already within the limit are returned as-is).
func resize(img image.Image, limit int) image.Image {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	longest := width
	if height > longest {
		longest = height
	}
	if longest <= limit {
		return img
	}
	scale := float64(limit) / float64(longest)
	targetWidth := max(1, int(float64(width)*scale+0.5))
	targetHeight := max(1, int(float64(height)*scale+0.5))
	target := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(target, target.Bounds(), img, bounds, draw.Over, nil)
	return target
}
