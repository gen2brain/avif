// Package avif implements an AVIF image decoder based on libavif compiled to WASM.
package avif

import (
	"image"
)

func init() {
	image.RegisterFormat("avif", "????ftypavif", Decode, DecodeConfig)
}
