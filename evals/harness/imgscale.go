package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"  // named: we call jpeg.Encode; also registers jpeg decoding
	_ "image/png" // blank: decode support only, we always encode out as jpeg
	"os"
)

// maxImageSide is the longest edge an image is downscaled to before it is sent to a
// vision model. The drill product photos in evals/assets are 3000-4000px on a side —
// sending them full-size roughly triples tokens/cost/latency for zero OCR benefit (the
// tariff infographic's smallest text is still legible well under this size). This is the
// eval's stand-in for the "downscale in code" step DECISIONS.md assigns to the real
// image adapter.
const maxImageSide = 1024

// loadAndScaleImage reads a png/jpeg file from disk and returns it as JPEG bytes, no
// larger than maxImageSide on its longest edge, plus the mimetype to send it as.
func loadAndScaleImage(path string) (data []byte, mimetype string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return nil, "", fmt.Errorf("decode %s: %w", path, err)
	}
	_ = format // png/jpeg both re-encoded as jpeg below; original format not otherwise needed

	scaled := downscale(img, maxImageSide)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, scaled, &jpeg.Options{Quality: 85}); err != nil {
		return nil, "", fmt.Errorf("encode %s: %w", path, err)
	}
	return buf.Bytes(), "image/jpeg", nil
}

// downscale box-filters img down to at most maxSide on its longest edge (a no-op if it
// already fits). Box filtering (averaging each output pixel over its source rectangle)
// keeps thin text strokes legible when SHRINKING, unlike nearest-neighbor which aliases
// small print into noise — the one property this eval actually depends on. Upscaling is
// never needed here, so only the shrink path is implemented.
func downscale(img image.Image, maxSide int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	longest := w
	if h > longest {
		longest = h
	}
	if longest <= maxSide {
		return img
	}
	scale := float64(maxSide) / float64(longest)
	newW := int(float64(w) * scale)
	newH := int(float64(h) * scale)
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	out := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for oy := 0; oy < newH; oy++ {
		srcY0 := b.Min.Y + int(float64(oy)*float64(h)/float64(newH))
		srcY1 := b.Min.Y + int(float64(oy+1)*float64(h)/float64(newH))
		if srcY1 <= srcY0 {
			srcY1 = srcY0 + 1
		}
		for ox := 0; ox < newW; ox++ {
			srcX0 := b.Min.X + int(float64(ox)*float64(w)/float64(newW))
			srcX1 := b.Min.X + int(float64(ox+1)*float64(w)/float64(newW))
			if srcX1 <= srcX0 {
				srcX1 = srcX0 + 1
			}
			out.Set(ox, oy, averageBox(img, srcX0, srcY0, srcX1, srcY1))
		}
	}
	return out
}

// averageBox returns the average color of img over [x0,x1) x [y0,y1).
func averageBox(img image.Image, x0, y0, x1, y1 int) color.RGBA {
	var rSum, gSum, bSum, aSum, n uint32
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			rSum += r
			gSum += g
			bSum += bl
			aSum += a
			n++
		}
	}
	if n == 0 {
		return color.RGBA{}
	}
	// RGBA() returns 16-bit-scaled components; shift back to 8-bit after averaging.
	return color.RGBA{
		R: uint8((rSum / n) >> 8),
		G: uint8((gSum / n) >> 8),
		B: uint8((bSum / n) >> 8),
		A: uint8((aSum / n) >> 8),
	}
}
