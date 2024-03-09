// Package avif implements an AVIF image decoder based on libavif compiled to WASM.
package avif

import (
	"errors"
	"image"
	"io"
)

// Errors .
var (
	ErrMemRead  = errors.New("avif: mem read failed")
	ErrMemWrite = errors.New("avif: mem write failed")
	ErrDecode   = errors.New("avif: decode failed")
)

// AVIF represents the possibly multiple images stored in a AVIF file.
type AVIF struct {
	// Decoded images, NRGBA or NRGBA64.
	Image []image.Image
	// Delay times, one per frame, in seconds.
	Delay []float64
}

// Decode reads a AVIF image from r and returns it as an image.Image.
func Decode(r io.Reader) (image.Image, error) {
	var err error
	var ret *AVIF

	if dynamic {
		ret, _, err = decodeDynamic(r, false, false)
		if err != nil {
			return nil, err
		}
	} else {
		ret, _, err = decode(r, false, false)
		if err != nil {
			return nil, err
		}
	}

	return ret.Image[0], nil
}

// DecodeConfig returns the color model and dimensions of a AVIF image without decoding the entire image.
func DecodeConfig(r io.Reader) (image.Config, error) {
	var err error
	var cfg image.Config

	if dynamic {
		_, cfg, err = decodeDynamic(r, true, false)
		if err != nil {
			return image.Config{}, err
		}
	} else {
		_, cfg, err = decode(r, true, false)
		if err != nil {
			return image.Config{}, err
		}
	}

	return cfg, nil
}

// DecodeAll reads a AVIF image from r and returns the sequential frames and timing information.
func DecodeAll(r io.Reader) (*AVIF, error) {
	var err error
	var ret *AVIF

	if dynamic {
		ret, _, err = decodeDynamic(r, false, true)
		if err != nil {
			return nil, err
		}
	} else {
		ret, _, err = decode(r, false, true)
		if err != nil {
			return nil, err
		}
	}

	return ret, nil
}

// Dynamic returns true when library is using the dynamic/shared library.
func Dynamic() bool {
	return dynamic
}

func init() {
	image.RegisterFormat("avif", "????ftypavif", Decode, DecodeConfig)
	image.RegisterFormat("avif", "????ftypavis", Decode, DecodeConfig)
}
