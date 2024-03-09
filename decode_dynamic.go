package avif

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
)

func decodeDynamic(r io.Reader, configOnly, decodeAll bool) (*AVIF, image.Config, error) {
	var cfg image.Config

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, cfg, err
	}

	decoder := avifDecoderCreate()
	decoder.IgnoreExif = 1
	decoder.IgnoreXMP = 1
	decoder.MaxThreads = int32(runtime.NumCPU())
	decoder.StrictFlags = 0

	defer avifDecoderDestroy(decoder)

	if !avifDecoderSetIOMemory(decoder, data) {
		return nil, cfg, fmt.Errorf("%w: %s", ErrDecode, string(decoder.Diag.Error[:]))
	}

	if !avifDecoderParse(decoder) {
		return nil, cfg, fmt.Errorf("%w: %s", ErrDecode, string(decoder.Diag.Error[:]))
	}

	cfg.Width = int(decoder.Image.Width)
	cfg.Height = int(decoder.Image.Height)

	cfg.ColorModel = color.RGBAModel
	if decoder.Image.Depth > 8 {
		cfg.ColorModel = color.RGBA64Model
	}

	if configOnly {
		return nil, cfg, nil
	}

	delay := make([]float64, 0)
	images := make([]image.Image, 0)

	var rgb avifRGBImage
	avifRGBImageSetDefaults(&rgb, decoder.Image)

	rgb.MaxThreads = int32(runtime.NumCPU())
	rgb.AlphaPremultiplied = 1
	if decoder.Image.Depth > 8 {
		rgb.Depth = 16
	}

	for avifDecoderNextImage(decoder) {
		if !avifRGBImageAllocatePixels(&rgb) {
			return nil, cfg, ErrDecode
		}

		if !avifImageYUVToRGB(decoder.Image, &rgb) {
			avifRGBImageFreePixels(&rgb)

			return nil, cfg, ErrDecode
		}

		size := int(rgb.RowBytes) * cfg.Height

		if decoder.Image.Depth > 8 {
			var b bytes.Buffer
			pix := unsafe.Slice((*uint16)(unsafe.Pointer(rgb.Pixels)), size/2)

			err = binary.Write(&b, binary.BigEndian, pix)
			if err != nil {
				return nil, cfg, nil
			}

			img := image.NewRGBA64(image.Rect(0, 0, cfg.Width, cfg.Height))
			img.Pix = b.Bytes()
			images = append(images, img)
		} else {
			img := image.NewRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
			copy(img.Pix, unsafe.Slice(rgb.Pixels, size))
			images = append(images, img)
		}

		avifRGBImageFreePixels(&rgb)

		delay = append(delay, decoder.ImageTiming.Duration)

		if !decodeAll {
			break
		}
	}

	runtime.KeepAlive(data)

	av := &AVIF{
		Image: images,
		Delay: delay,
	}

	return av, cfg, nil
}

func init() {
	var err error

	libavif, err = loadLibrary()
	if err == nil {
		dynamic = true
	} else {
		return
	}

	purego.RegisterLibFunc(&_avifDecoderCreate, libavif, "avifDecoderCreate")
	purego.RegisterLibFunc(&_avifDecoderDestroy, libavif, "avifDecoderDestroy")
	purego.RegisterLibFunc(&_avifDecoderSetIOMemory, libavif, "avifDecoderSetIOMemory")
	purego.RegisterLibFunc(&_avifDecoderParse, libavif, "avifDecoderParse")
	purego.RegisterLibFunc(&_avifDecoderNextImage, libavif, "avifDecoderNextImage")
	purego.RegisterLibFunc(&_avifRGBImageSetDefaults, libavif, "avifRGBImageSetDefaults")
	purego.RegisterLibFunc(&_avifRGBImageAllocatePixels, libavif, "avifRGBImageAllocatePixels")
	purego.RegisterLibFunc(&_avifRGBImageFreePixels, libavif, "avifRGBImageFreePixels")
	purego.RegisterLibFunc(&_avifImageYUVToRGB, libavif, "avifImageYUVToRGB")
}

var (
	libavif uintptr
	dynamic bool
)

var (
	_avifDecoderCreate          func() *avifDecoder
	_avifDecoderDestroy         func(*avifDecoder)
	_avifDecoderSetIOMemory     func(*avifDecoder, []byte, uint64) int
	_avifDecoderParse           func(*avifDecoder) int
	_avifDecoderNextImage       func(*avifDecoder) int
	_avifRGBImageSetDefaults    func(*avifRGBImage, *avifImage)
	_avifRGBImageAllocatePixels func(*avifRGBImage) int
	_avifRGBImageFreePixels     func(*avifRGBImage)
	_avifImageYUVToRGB          func(*avifImage, *avifRGBImage) int
)

func avifDecoderCreate() *avifDecoder {
	return _avifDecoderCreate()
}

func avifDecoderDestroy(decoder *avifDecoder) {
	_avifDecoderDestroy(decoder)
}

func avifDecoderSetIOMemory(decoder *avifDecoder, data []byte) bool {
	ret := _avifDecoderSetIOMemory(decoder, data, uint64(len(data)))
	return ret == 0
}

func avifDecoderParse(decoder *avifDecoder) bool {
	ret := _avifDecoderParse(decoder)
	return ret == 0
}

func avifDecoderNextImage(decoder *avifDecoder) bool {
	ret := _avifDecoderNextImage(decoder)
	return ret == 0
}

func avifRGBImageSetDefaults(rgb *avifRGBImage, img *avifImage) {
	_avifRGBImageSetDefaults(rgb, img)
}

func avifRGBImageAllocatePixels(rgb *avifRGBImage) bool {
	ret := _avifRGBImageAllocatePixels(rgb)
	return ret == 0
}

func avifRGBImageFreePixels(rgb *avifRGBImage) {
	_avifRGBImageFreePixels(rgb)
}

func avifImageYUVToRGB(img *avifImage, rgb *avifRGBImage) bool {
	ret := _avifImageYUVToRGB(img, rgb)
	return ret == 0
}

const (
	avifPixelFormatYuv444 = 1
	avifPixelFormatYuv422 = 2
	avifPixelFormatYuv420 = 3
)

type avifImage struct {
	Width                   uint32
	Height                  uint32
	Depth                   uint32
	YuvFormat               uint32
	YuvRange                uint32
	YuvChromaSamplePosition uint32
	YuvPlanes               [3]*uint8
	YuvRowBytes             [3]uint32
	ImageOwnsYUVPlanes      int32
	AlphaPlane              *uint8
	AlphaRowBytes           uint32
	ImageOwnsAlphaPlane     int32
	AlphaPremultiplied      int32
	Icc                     avifRWData
	ColorPrimaries          uint16
	TransferCharacteristics uint16
	MatrixCoefficients      uint16
	Clli                    avifContentLightLevelInformationBox
	TransformFlags          uint32
	Pasp                    avifPixelAspectRatioBox
	Clap                    avifCleanApertureBox
	Irot                    avifImageRotation
	Imir                    avifImageMirror
	Exif                    avifRWData
	Xmp                     avifRWData
}

type avifImageTiming struct {
	Timescale            uint64
	Pts                  float64
	PtsInTimescales      uint64
	Duration             float64
	DurationInTimescales uint64
}

type avifIO struct {
	Destroy    *[0]byte
	Read       *[0]byte
	Write      *[0]byte
	SizeHint   uint64
	Persistent int32
	Data       *byte
}

type avifIOStats struct {
	ColorOBUSize uint64
	AlphaOBUSize uint64
}

type avifDiagnostics struct {
	Error [256]uint8
}

type avifRGBImage struct {
	Width              uint32
	Height             uint32
	Depth              uint32
	Format             uint32
	ChromaUpsampling   uint32
	ChromaDownsampling uint32
	AvoidLibYUV        int32
	IgnoreAlpha        int32
	AlphaPremultiplied int32
	IsFloat            int32
	MaxThreads         int32
	Pixels             *uint8
	RowBytes           uint32
	_                  [4]byte
}

type avifRWData struct {
	Data *uint8
	Size uint64
}

type avifContentLightLevelInformationBox struct {
	MaxCLL  uint16
	MaxPALL uint16
}

type avifPixelAspectRatioBox struct {
	HSpacing uint32
	VSpacing uint32
}

type avifCleanApertureBox struct {
	WidthN    uint32
	WidthD    uint32
	HeightN   uint32
	HeightD   uint32
	HorizOffN uint32
	HorizOffD uint32
	VertOffN  uint32
	VertOffD  uint32
}

type avifImageRotation struct {
	Angle uint8
}

type avifImageMirror struct {
	Axis uint8
}

type avifDecoderData struct{}

type avifDecoder struct {
	CodecChoice          uint32
	MaxThreads           int32
	RequestedSource      uint32
	AllowProgressive     int32
	AllowIncremental     int32
	IgnoreExif           int32
	IgnoreXMP            int32
	ImageSizeLimit       uint32
	ImageDimensionLimit  uint32
	ImageCountLimit      uint32
	StrictFlags          uint32
	Image                *avifImage
	ImageIndex           int32
	ImageCount           int32
	ProgressiveState     uint32
	ImageTiming          avifImageTiming
	Timescale            uint64
	Duration             float64
	DurationInTimescales uint64
	RepetitionCount      int32
	AlphaPresent         int32
	IoStats              avifIOStats
	Diag                 avifDiagnostics
	Io                   *avifIO
	Data                 *avifDecoderData
}
