// Package avif implements an AVIF image decoder based on libavif compiled to WASM.
package avif

import (
	"errors"
	"image"
	"image/draw"
	"io"
)

// Errors .
var (
	ErrMemRead  = errors.New("avif: mem read failed")
	ErrMemWrite = errors.New("avif: mem write failed")
	ErrDecode   = errors.New("avif: decode failed")
	ErrEncode   = errors.New("avif: encode failed")
)

// AVIF represents the possibly multiple images stored in a AVIF file.
type AVIF struct {
	// Decoded images, NRGBA or NRGBA64.
	Image []image.Image
	// Delay times, one per frame, in seconds.
	Delay []float64
}

// DefaultQuality is the default quality encoding parameter.
const DefaultQuality = 60

// DefaultSpeed is the default speed encoding parameter.
const DefaultSpeed = 10

// Options are the encoding parameters.
type Options struct {
	// Quality in the range [0,100]. Quality of 100 implies lossless. Default is 60.
	Quality int
	// Quality in the range [0,100].
	QualityAlpha int
	// Speed in the range [0,10]. Slower should make for a better quality image in less bytes.
	Speed int
	// Chroma subsampling, 444|422|420.
	ChromaSubsampling image.YCbCrSubsampleRatio
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

// Encode writes the image m to w with the given options.
func Encode(w io.Writer, m image.Image, o ...Options) error {
	quality := DefaultQuality
	qualityAlpha := DefaultQuality
	speed := DefaultSpeed
	chroma := image.YCbCrSubsampleRatio420

	if o != nil {
		opt := o[0]
		quality = opt.Quality
		qualityAlpha = opt.QualityAlpha
		speed = opt.Speed
		chroma = opt.ChromaSubsampling

		if quality <= 0 {
			quality = DefaultQuality
		} else if quality > 100 {
			quality = 100
		}

		if qualityAlpha <= 0 {
			qualityAlpha = DefaultQuality
		} else if qualityAlpha > 100 {
			qualityAlpha = 100
		}

		if speed <= 0 {
			speed = DefaultSpeed
		} else if speed > 10 {
			speed = 10
		}
	}

	if dynamic {
		err := encodeDynamic(w, m, quality, qualityAlpha, speed, chroma)
		if err != nil {
			return err
		}
	} else {
		err := encode(w, m, quality, qualityAlpha, speed, chroma)
		if err != nil {
			return err
		}
	}

	return nil
}

// Dynamic returns error (if there was any) during opening dynamic/shared library.
func Dynamic() error {
	return dynamicErr
}

// InitDecoder initializes wazero runtime and compiles the module.
// This function does nothing if a dynamic/shared library is used and Dynamic() returns nil.
// There is no need to explicitly call this function, first Decode will initialize the runtime.
func InitDecoder() {
	if dynamic && dynamicErr == nil {
		return
	}

	initDecoderOnce()
}

// InitEncoder initializes wazero runtime and compiles the module.
// This function does nothing if a dynamic/shared library is used and Dynamic() returns nil.
// There is no need to explicitly call this function, first Encode will initialize the runtime.
func InitEncoder() {
	if dynamic && dynamicErr == nil {
		return
	}

	initEncoderOnce()
}

const (
	avifChromaUpsamplingFastest = 1

	avifPixelFormatYuv444 = 1
	avifPixelFormatYuv422 = 2
	avifPixelFormatYuv420 = 3

	avifAddImageFlagSingle = 2
)

func imageToRGBA(src image.Image) *image.RGBA {
	if dst, ok := src.(*image.RGBA); ok {
		return dst
	}

	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)

	return dst
}

func init() {
	image.RegisterFormat("avif", "????ftypavif", Decode, DecodeConfig)
	image.RegisterFormat("avif", "????ftypavis", Decode, DecodeConfig)
}
