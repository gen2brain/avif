// Package avif implements an AVIF image decoder based on libavif compiled to WASM.
package avif

import (
	"image"
)

// AVIF represents the possibly multiple images stored in a AVIF file.
type AVIF struct {
	// Decoded images, NRGBA or NRGBA64.
	Image []image.Image
	// Delay times, one per frame, in seconds.
	Delay []float64
}

func init() {
	image.RegisterFormat("avif", "????ftypavif", Decode, DecodeConfig)
	image.RegisterFormat("avif", "????ftypavis", Decode, DecodeConfig)
}
