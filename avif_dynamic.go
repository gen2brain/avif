//go:build (unix || darwin || windows) && !nodynamic

package avif

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"runtime"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"
)

func decodeDynamic(r io.Reader, configOnly, decodeAll bool) (*AVIF, image.Config, error) {
	var err error
	var cfg image.Config
	var data []byte

	data, err = io.ReadAll(r)
	if err != nil {
		return nil, cfg, fmt.Errorf("read: %w", err)
	}

	decoder := avifDecoderCreate()
	decoder.IgnoreExif = 1
	decoder.IgnoreXMP = 1
	decoder.MaxThreads = int32(runtime.NumCPU())
	decoder.StrictFlags = 0

	defer avifDecoderDestroy(decoder)

	if !avifDecoderSetIOMemory(decoder, data) {
		return nil, cfg, fmt.Errorf("%w: %s", ErrDecode, toStr(decoder.Diag))
	}

	if !avifDecoderParse(decoder) {
		return nil, cfg, fmt.Errorf("%w: %s", ErrDecode, toStr(decoder.Diag))
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

	if decoder.ImageCount > 1 && decodeAll {
		rgb.ChromaUpsampling = avifChromaUpsamplingFastest
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

func encodeDynamic(w io.Writer, m image.Image, quality, qualityAlpha, speed int, subsampleRatio image.YCbCrSubsampleRatio) error {
	i := imageToRGBA(m)

	var chroma int
	switch subsampleRatio {
	case image.YCbCrSubsampleRatio444:
		chroma = avifPixelFormatYuv444
	case image.YCbCrSubsampleRatio422:
		chroma = avifPixelFormatYuv422
	case image.YCbCrSubsampleRatio420:
		chroma = avifPixelFormatYuv420
	default:
		return fmt.Errorf("unsupported chroma %d", subsampleRatio)
	}

	img := avifImageCreate(i.Bounds().Dx(), i.Bounds().Dy(), 8, chroma)
	defer avifImageDestroy(img)

	var rgb avifRGBImage
	avifRGBImageSetDefaults(&rgb, img)

	rgb.MaxThreads = int32(runtime.NumCPU())
	rgb.AlphaPremultiplied = 1

	if !avifRGBImageAllocatePixels(&rgb) {
		return ErrEncode
	}
	defer avifRGBImageFreePixels(&rgb)

	copy(unsafe.Slice(rgb.Pixels, rgb.RowBytes*rgb.Height), i.Pix)

	if !avifImageRGBToYuv(img, &rgb) {
		return ErrEncode
	}

	var output avifRWData
	defer avifRWDataFree(&output)

	encoder := avifEncoderCreate()
	defer avifEncoderDestroy(encoder)

	encoder.MaxThreads = int32(runtime.NumCPU())
	encoder.Quality = int32(quality)
	encoder.QualityAlpha = int32(qualityAlpha)
	encoder.Speed = int32(speed)

	if !avifEncoderAddImage(encoder, img, 1, avifAddImageFlagSingle) {
		return ErrEncode
	}

	if !avifEncoderFinish(encoder, &output) {
		return ErrEncode
	}

	_, err := w.Write(unsafe.Slice(output.Data, output.Size))
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func init() {
	var err error
	defer func() {
		if r := recover(); r != nil {
			dynamic = false
			dynamicErr = fmt.Errorf("%v", r)
		}
	}()

	libavif, err = loadLibrary()
	if err == nil {
		dynamic = true
	} else {
		dynamicErr = err

		return
	}

	purego.RegisterLibFunc(&_avifVersion, libavif, "avifVersion")
	purego.RegisterLibFunc(&_avifDecoderCreate, libavif, "avifDecoderCreate")
	purego.RegisterLibFunc(&_avifDecoderDestroy, libavif, "avifDecoderDestroy")
	purego.RegisterLibFunc(&_avifDecoderSetIOMemory, libavif, "avifDecoderSetIOMemory")
	purego.RegisterLibFunc(&_avifDecoderParse, libavif, "avifDecoderParse")
	purego.RegisterLibFunc(&_avifDecoderNextImage, libavif, "avifDecoderNextImage")
	purego.RegisterLibFunc(&_avifRGBImageSetDefaults, libavif, "avifRGBImageSetDefaults")
	purego.RegisterLibFunc(&_avifRGBImageAllocatePixels, libavif, "avifRGBImageAllocatePixels")
	purego.RegisterLibFunc(&_avifRGBImageFreePixels, libavif, "avifRGBImageFreePixels")
	purego.RegisterLibFunc(&_avifImageYUVToRGB, libavif, "avifImageYUVToRGB")
	purego.RegisterLibFunc(&_avifImageRGBToYUV, libavif, "avifImageRGBToYUV")
	purego.RegisterLibFunc(&_avifImageCreate, libavif, "avifImageCreate")
	purego.RegisterLibFunc(&_avifImageDestroy, libavif, "avifImageDestroy")
	purego.RegisterLibFunc(&_avifEncoderCreate, libavif, "avifEncoderCreate")
	purego.RegisterLibFunc(&_avifEncoderDestroy, libavif, "avifEncoderDestroy")
	purego.RegisterLibFunc(&_avifEncoderAddImage, libavif, "avifEncoderAddImage")
	purego.RegisterLibFunc(&_avifEncoderFinish, libavif, "avifEncoderFinish")
	purego.RegisterLibFunc(&_avifRWDataFree, libavif, "avifRWDataFree")

	major, _ := avifVersion()
	if major < 1 {
		dynamic = false
		dynamicErr = fmt.Errorf("minimum required libavif version is 1.0.0")
	}
}

var (
	libavif    uintptr
	dynamic    bool
	dynamicErr error
)

var (
	_avifVersion                func() string
	_avifDecoderCreate          func() *avifDecoder
	_avifDecoderDestroy         func(*avifDecoder)
	_avifDecoderSetIOMemory     func(*avifDecoder, []byte, uint64) int
	_avifDecoderParse           func(*avifDecoder) int
	_avifDecoderNextImage       func(*avifDecoder) int
	_avifRGBImageSetDefaults    func(*avifRGBImage, *avifImage)
	_avifRGBImageAllocatePixels func(*avifRGBImage) int
	_avifRGBImageFreePixels     func(*avifRGBImage)
	_avifImageYUVToRGB          func(*avifImage, *avifRGBImage) int
	_avifImageRGBToYUV          func(*avifImage, *avifRGBImage) int
	_avifImageCreate            func(int, int, int, int) *avifImage
	_avifImageDestroy           func(*avifImage)
	_avifEncoderCreate          func() *avifEncoder
	_avifEncoderDestroy         func(*avifEncoder)
	_avifEncoderAddImage        func(*avifEncoder, *avifImage, uint64, int) int
	_avifEncoderFinish          func(*avifEncoder, *avifRWData) int
	_avifRWDataFree             func(*avifRWData)
)

func avifVersion() (int, int) {
	var major, minor, patch int

	version := _avifVersion()
	_, _ = fmt.Sscanf(version, "%d.%d.%d", &major, &minor, &patch)

	return major, minor
}

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

func avifImageRGBToYuv(img *avifImage, rgb *avifRGBImage) bool {
	ret := _avifImageRGBToYUV(img, rgb)
	return ret == 0
}

func avifImageCreate(width, height, depth, format int) *avifImage {
	return _avifImageCreate(width, height, depth, format)
}

func avifImageDestroy(img *avifImage) {
	_avifImageDestroy(img)
}

func avifEncoderCreate() *avifEncoder {
	return _avifEncoderCreate()
}

func avifEncoderDestroy(encoder *avifEncoder) {
	_avifEncoderDestroy(encoder)
}

func avifEncoderAddImage(encoder *avifEncoder, img *avifImage, durationInTimescales uint64, flags int) bool {
	ret := _avifEncoderAddImage(encoder, img, durationInTimescales, flags)
	return ret == 0
}

func avifEncoderFinish(encoder *avifEncoder, output *avifRWData) bool {
	ret := _avifEncoderFinish(encoder, output)
	return ret == 0
}

func avifRWDataFree(output *avifRWData) {
	_avifRWDataFree(output)
}

func toStr(diagnostics avifDiagnostics) string {
	str := string(diagnostics.Error[:])
	idx := strings.Index(str, "\x00")
	if idx != -1 {
		str = str[:idx]
	}

	return strings.TrimSpace(str)
}

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

type avifEncoderData struct{}

type avifCodecSpecificOptions struct{}

type avifScalingMode struct {
	Horizontal avifFraction
	Vertical   avifFraction
}

type avifFraction struct {
	N int32
	D int32
}

type avifEncoder struct {
	CodecChoice       uint32
	MaxThreads        int32
	Speed             int32
	KeyframeInterval  int32
	Timescale         uint64
	RepetitionCount   int32
	ExtraLayerCount   uint32
	Quality           int32
	QualityAlpha      int32
	MinQuantizer      int32
	MaxQuantizer      int32
	MinQuantizerAlpha int32
	MaxQuantizerAlpha int32
	TileRowsLog2      int32
	TileColsLog2      int32
	AutoTiling        int32
	ScalingMode       avifScalingMode
	IoStats           avifIOStats
	Diag              avifDiagnostics
	Data              *avifEncoderData
	CsOptions         *avifCodecSpecificOptions
}
