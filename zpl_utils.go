package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
)

// imageToZPL converts an image to ZPL ^GFA command
// This is a basic implementation without compression
func imageToZPL(img image.Image) []byte {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate row width in bytes (rounded up)
	rowBytes := (width + 7) / 8
	totalBytes := rowBytes * height

	var sb bytes.Buffer
	// ^XA: Start Format
	// ^FO0,0: Field Origin 0,0
	// ^GFA: Graphic Field (A compression - but we use hex which is uncompressed effectively in A mode? No, A is ASCII Hex)
	// format: ^GFA,a,b,c,data
	// a = binary data total bytes
	// b = total bytes of graphic field
	// c = bytes per row
	sb.WriteString(fmt.Sprintf("^XA^FO0,0^GFA,%d,%d,%d,", totalBytes, totalBytes, rowBytes))

	for y := 0; y < height; y++ {
		var b byte
		for x := 0; x < width; x++ {
			// Get pixel
			c := img.At(x, y)
			// Convert to gray
			gray := color.GrayModel.Convert(c).(color.Gray)

			// Thresholding
			// In ZPL: 1 is black (print), 0 is white (no print)
			// In Gray: 0 is black, 255 is white
			// So if pixel is dark (<128), we set bit to 1
			if gray.Y < 128 {
				b |= 1 << (7 - (x % 8))
			}

			// If end of byte or end of row
			if x%8 == 7 || x == width-1 {
				// Append hex
				sb.WriteString(fmt.Sprintf("%02X", b))
				b = 0
			}
		}
		// No newline needed for ZPL data, but helpful for debug? ZPL ignores whitespace in data?
		// Actually strictly speaking data should be continuous.
	}

	sb.WriteString("^FS^XZ")
	return sb.Bytes()
}
