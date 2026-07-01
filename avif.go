// Package avif implements an AVIF image decoder based on libavif compiled to WASM.
package avif

//go:generate wasm2go -pkg avif -unsafe -tags wasm2go -o libavif.go lib/avif.wasm

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
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
	// Quality in the range [0,100]. Default is 60.
	Quality int
	// Quality in the range [0,100].
	QualityAlpha int
	// Speed in the range [0,10]. Slower should make for a better quality image in less bytes.
	Speed int
	// Chroma subsampling, 444|422|420.
	ChromaSubsampling image.YCbCrSubsampleRatio
	// Lossless enables lossless compression. Lossless ignores quality and forces 4:4:4 chroma.
	Lossless bool
	// AutoRotate applies the irot/imir orientation to the decoded image (Decode/DecodeAll only).
	AutoRotate bool
}

// avifMaxHeaderSize bounds the prefix read to find dimensions without decoding.
const avifMaxHeaderSize = 1 << 18

func doDecode(r io.Reader, configOnly, decodeAll bool) (*AVIF, image.Config, error) {
	if dynamic {
		return decodeDynamic(r, configOnly, decodeAll)
	}

	return decode(r, configOnly, decodeAll)
}

// Decode reads a AVIF image from r; pass Options{AutoRotate: true} to apply the orientation.
func Decode(r io.Reader, opts ...Options) (image.Image, error) {
	if len(opts) > 0 && opts[0].AutoRotate {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("avif: read: %w", err)
		}

		ret, _, err := doDecode(bytes.NewReader(data), false, false)
		if err != nil {
			return nil, err
		}

		props, _ := parseAVIFProps(data)

		return applyOrientation(ret.Image[0], props.orientation), nil
	}

	ret, _, err := doDecode(r, false, false)
	if err != nil {
		return nil, err
	}

	return ret.Image[0], nil
}

// DecodeConfig returns the color model and dimensions of a AVIF image without decoding the entire image.
func DecodeConfig(r io.Reader) (image.Config, error) {
	prefix, err := io.ReadAll(io.LimitReader(r, avifMaxHeaderSize))
	if err != nil {
		return image.Config{}, fmt.Errorf("avif: read: %w", err)
	}

	if props, ok := parseAVIFProps(prefix); ok {
		cm := color.RGBAModel
		if props.hiDepth {
			cm = color.RGBA64Model
		}

		return image.Config{ColorModel: cm, Width: props.width, Height: props.height}, nil
	}

	_, cfg, err := doDecode(io.MultiReader(bytes.NewReader(prefix), r), true, false)
	if err != nil {
		return image.Config{}, err
	}

	return cfg, nil
}

// DecodeAll reads a AVIF image from r; pass Options{AutoRotate: true} to orient each frame.
func DecodeAll(r io.Reader, opts ...Options) (*AVIF, error) {
	if len(opts) > 0 && opts[0].AutoRotate {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("avif: read: %w", err)
		}

		ret, _, err := doDecode(bytes.NewReader(data), false, true)
		if err != nil {
			return nil, err
		}

		props, _ := parseAVIFProps(data)
		if props.orientation > 1 {
			for i := range ret.Image {
				ret.Image[i] = applyOrientation(ret.Image[i], props.orientation)
			}
		}

		return ret, nil
	}

	ret, _, err := doDecode(r, false, true)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// Encode writes the image m to w with the given options.
func Encode(w io.Writer, m image.Image, o ...Options) error {
	quality := DefaultQuality
	qualityAlpha := DefaultQuality
	speed := DefaultSpeed
	chroma := image.YCbCrSubsampleRatio420
	lossless := false

	if o != nil {
		opt := o[0]
		quality = opt.Quality
		qualityAlpha = opt.QualityAlpha
		speed = opt.Speed
		chroma = opt.ChromaSubsampling
		lossless = opt.Lossless

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

		if speed < 0 {
			speed = DefaultSpeed
		} else if speed > 10 {
			speed = 10
		}
	}

	if lossless {
		quality = 100
		qualityAlpha = 100
		chroma = image.YCbCrSubsampleRatio444
	}

	if dynamic {
		err := encodeDynamic(w, m, quality, qualityAlpha, speed, chroma, lossless)
		if err != nil {
			return err
		}
	} else {
		err := encode(w, m, quality, qualityAlpha, speed, chroma, lossless)
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

const (
	avifChromaUpsamplingFastest = 1

	avifPixelFormatYuv444 = 1
	avifPixelFormatYuv422 = 2
	avifPixelFormatYuv420 = 3

	avifAddImageFlagSingle = 2

	avifMatrixCoefficientsIdentity = 0
	avifRangeFull                  = 1
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

func decodeWrapper(r io.Reader) (image.Image, error) {
	return Decode(r)
}

func init() {
	image.RegisterFormat("avif", "????ftypavif", decodeWrapper, DecodeConfig)
	image.RegisterFormat("avif", "????ftypavis", decodeWrapper, DecodeConfig)
}
