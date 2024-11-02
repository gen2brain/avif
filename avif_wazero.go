package avif

import (
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"sync"
	"unsafe"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed lib/decode.wasm.gz
var decodeWasm []byte

//go:embed lib/encode.wasm.gz
var encodeWasm []byte

func decode(r io.Reader, configOnly, decodeAll bool) (*AVIF, image.Config, error) {
	initDecoderOnce()

	var cfg image.Config
	var data []byte

	ctx := context.Background()
	dec, err := rtd.InstantiateModule(ctx, cmd, mc)
	if err != nil {
		return nil, cfg, err
	}

	defer dec.Close(ctx)

	_alloc := dec.ExportedFunction("malloc")
	_free := dec.ExportedFunction("free")
	_decode := dec.ExportedFunction("decode")

	data, err = io.ReadAll(r)
	if err != nil {
		return nil, cfg, fmt.Errorf("read: %w", err)
	}

	inSize := len(data)

	res, err := _alloc.Call(ctx, uint64(inSize))
	if err != nil {
		return nil, cfg, fmt.Errorf("alloc: %w", err)
	}
	inPtr := res[0]
	defer _free.Call(ctx, inPtr)

	ok := dec.Memory().Write(uint32(inPtr), data)
	if !ok {
		return nil, cfg, ErrMemWrite
	}

	res, err = _alloc.Call(ctx, 4*4)
	if err != nil {
		return nil, cfg, fmt.Errorf("alloc: %w", err)
	}
	defer _free.Call(ctx, res[0])

	widthPtr := res[0]
	heightPtr := res[0] + 4
	depthPtr := res[0] + 8
	countPtr := res[0] + 12

	res, err = _decode.Call(ctx, inPtr, uint64(inSize), 1, 0, widthPtr, heightPtr, depthPtr, countPtr, 0, 0)
	if err != nil {
		return nil, cfg, fmt.Errorf("decode: %w", err)
	}

	if res[0] == 0 {
		return nil, cfg, ErrDecode
	}

	width, ok := dec.Memory().ReadUint32Le(uint32(widthPtr))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	height, ok := dec.Memory().ReadUint32Le(uint32(heightPtr))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	depth, ok := dec.Memory().ReadUint32Le(uint32(depthPtr))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	count, ok := dec.Memory().ReadUint32Le(uint32(countPtr))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	cfg.Width = int(width)
	cfg.Height = int(height)

	cfg.ColorModel = color.RGBAModel
	if depth > 8 {
		cfg.ColorModel = color.RGBA64Model
	}

	if configOnly {
		return nil, cfg, nil
	}

	size := cfg.Width * cfg.Height * 4
	if depth > 8 {
		size = cfg.Width * cfg.Height * 8
	}

	outSize := size
	if decodeAll {
		outSize = size * int(count)
	}

	res, err = _alloc.Call(ctx, uint64(outSize))
	if err != nil {
		return nil, cfg, fmt.Errorf("alloc: %w", err)
	}
	outPtr := res[0]
	defer _free.Call(ctx, outPtr)

	delaySize := 8
	if decodeAll {
		delaySize = 8 * int(count)
	}

	res, err = _alloc.Call(ctx, uint64(delaySize))
	if err != nil {
		return nil, cfg, fmt.Errorf("alloc: %w", err)
	}
	delayPtr := res[0]
	defer _free.Call(ctx, delayPtr)

	all := 0
	if decodeAll {
		all = 1
	}

	res, err = _decode.Call(ctx, inPtr, uint64(inSize), 0, uint64(all), widthPtr, heightPtr, depthPtr, countPtr, delayPtr, outPtr)
	if err != nil {
		return nil, cfg, fmt.Errorf("decode: %w", err)
	}

	if res[0] == 0 {
		return nil, cfg, ErrDecode
	}

	delay := make([]float64, 0)
	images := make([]image.Image, 0)

	for i := 0; i < int(count); i++ {
		out, ok := dec.Memory().Read(uint32(outPtr)+uint32(i*size), uint32(size))
		if !ok {
			return nil, cfg, ErrMemRead
		}

		if depth > 8 {
			var b bytes.Buffer
			pix := unsafe.Slice((*uint16)(unsafe.Pointer(&out[0])), size/2)

			err = binary.Write(&b, binary.BigEndian, pix)
			if err != nil {
				return nil, cfg, nil
			}

			img := image.NewRGBA64(image.Rect(0, 0, cfg.Width, cfg.Height))
			img.Pix = b.Bytes()
			images = append(images, img)
		} else {
			img := image.NewRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
			img.Pix = out
			images = append(images, img)
		}

		d, ok := dec.Memory().ReadFloat64Le(uint32(delayPtr) + uint32(i*8))
		if !ok {
			return nil, cfg, ErrMemRead
		}

		delay = append(delay, d)

		if !decodeAll {
			break
		}
	}

	ret := &AVIF{
		Image: images,
		Delay: delay,
	}

	return ret, cfg, nil
}

func encode(w io.Writer, m image.Image, quality, qualityAlpha, speed int, subsampleRatio image.YCbCrSubsampleRatio) error {
	initEncoderOnce()

	ctx := context.Background()

	enc, err := rte.InstantiateModule(ctx, cme, mc)
	if err != nil {
		return err
	}

	defer enc.Close(ctx)

	_alloc := enc.ExportedFunction("malloc")
	_free := enc.ExportedFunction("free")
	_encode := enc.ExportedFunction("encode")

	img := imageToRGBA(m)

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

	res, err := _alloc.Call(ctx, uint64(len(img.Pix)))
	if err != nil {
		return fmt.Errorf("alloc: %w", err)
	}
	inPtr := res[0]
	defer _free.Call(ctx, inPtr)

	ok := enc.Memory().Write(uint32(inPtr), img.Pix)
	if !ok {
		return ErrMemWrite
	}

	res, err = _alloc.Call(ctx, 8)
	if err != nil {
		return fmt.Errorf("alloc: %w", err)
	}
	sizePtr := res[0]
	defer _free.Call(ctx, sizePtr)

	res, err = _encode.Call(ctx, inPtr, uint64(img.Bounds().Dx()), uint64(img.Bounds().Dy()), sizePtr, uint64(quality),
		uint64(qualityAlpha), uint64(speed), uint64(chroma))
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	size, ok := enc.Memory().ReadUint64Le(uint32(sizePtr))
	if !ok {
		return ErrMemRead
	}

	if size == 0 {
		return ErrEncode
	}

	defer _free.Call(ctx, res[0])

	out, ok := enc.Memory().Read(uint32(res[0]), uint32(size))
	if !ok {
		return ErrMemRead
	}

	_, err = w.Write(out)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

var (
	rtd wazero.Runtime
	rte wazero.Runtime
	cmd wazero.CompiledModule
	cme wazero.CompiledModule

	mc = wazero.NewModuleConfig().WithStderr(os.Stderr).WithStdout(os.Stdout)

	initDecoderOnce = sync.OnceFunc(initializeDecoder)
	initEncoderOnce = sync.OnceFunc(initializeEncoder)
)

func initializeDecoder() {
	ctx := context.Background()
	rtd = wazero.NewRuntime(ctx)

	r, err := gzip.NewReader(bytes.NewReader(decodeWasm))
	if err != nil {
		panic(err)
	}
	defer r.Close()

	var data bytes.Buffer
	_, err = data.ReadFrom(r)
	if err != nil {
		panic(err)
	}

	cmd, err = rtd.CompileModule(ctx, data.Bytes())
	if err != nil {
		panic(err)
	}

	wasi_snapshot_preview1.MustInstantiate(ctx, rtd)
}

func initializeEncoder() {
	ctx := context.Background()
	rte = wazero.NewRuntime(ctx)

	r, err := gzip.NewReader(bytes.NewReader(encodeWasm))
	if err != nil {
		panic(err)
	}
	defer r.Close()

	var data bytes.Buffer
	_, err = data.ReadFrom(r)
	if err != nil {
		panic(err)
	}

	cme, err = rte.CompileModule(ctx, data.Bytes())
	if err != nil {
		panic(err)
	}

	wasi_snapshot_preview1.MustInstantiate(ctx, rte)
}
