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
	"sync/atomic"
	"unsafe"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed lib/avif.wasm.gz
var avifWasm []byte

func decode(r io.Reader, configOnly, decodeAll bool) (*AVIF, image.Config, error) {
	if !initialized.Load() {
		initialize()
	}

	var cfg image.Config
	var avif bytes.Buffer

	_, err := avif.ReadFrom(r)
	if err != nil {
		return nil, cfg, fmt.Errorf("read: %w", err)
	}

	inSize := avif.Len()
	ctx := context.Background()

	res, err := _alloc.Call(ctx, uint64(inSize))
	if err != nil {
		return nil, cfg, fmt.Errorf("alloc: %w", err)
	}
	inPtr := res[0]
	defer _free.Call(ctx, inPtr)

	ok := mod.Memory().Write(uint32(inPtr), avif.Bytes())
	if !ok {
		return nil, cfg, ErrMemWrite
	}

	res, err = _alloc.Call(ctx, 4)
	if err != nil {
		return nil, cfg, fmt.Errorf("alloc: %w", err)
	}
	widthPtr := res[0]
	defer _free.Call(ctx, widthPtr)

	res, err = _alloc.Call(ctx, 4)
	if err != nil {
		return nil, cfg, fmt.Errorf("alloc: %w", err)
	}
	heightPtr := res[0]
	defer _free.Call(ctx, heightPtr)

	res, err = _alloc.Call(ctx, 4)
	if err != nil {
		return nil, cfg, fmt.Errorf("alloc: %w", err)
	}
	depthPtr := res[0]
	defer _free.Call(ctx, depthPtr)

	res, err = _alloc.Call(ctx, 4)
	if err != nil {
		return nil, cfg, fmt.Errorf("alloc: %w", err)
	}
	countPtr := res[0]
	defer _free.Call(ctx, countPtr)

	res, err = _decode.Call(ctx, inPtr, uint64(inSize), 1, 0, widthPtr, heightPtr, depthPtr, countPtr, 0, 0)
	if err != nil {
		return nil, cfg, fmt.Errorf("decode: %w", err)
	}

	if res[0] == 0 {
		return nil, cfg, ErrDecode
	}

	width, ok := mod.Memory().ReadUint32Le(uint32(widthPtr))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	height, ok := mod.Memory().ReadUint32Le(uint32(heightPtr))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	depth, ok := mod.Memory().ReadUint32Le(uint32(depthPtr))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	count, ok := mod.Memory().ReadUint32Le(uint32(countPtr))
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
		out, ok := mod.Memory().Read(uint32(outPtr)+uint32(i*size), uint32(size))
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

		d, ok := mod.Memory().ReadFloat64Le(uint32(delayPtr) + uint32(i*8))
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

func encode(w io.Writer, m image.Image, quality, qualityAlpha, speed int) error {
	if !initialized.Load() {
		initialize()
	}

	img := imageToRGBA(m)
	ctx := context.Background()

	res, err := _alloc.Call(ctx, uint64(len(img.Pix)))
	if err != nil {
		return fmt.Errorf("alloc: %w", err)
	}
	inPtr := res[0]
	defer _free.Call(ctx, inPtr)

	ok := mod.Memory().Write(uint32(inPtr), img.Pix)
	if !ok {
		return ErrMemWrite
	}

	res, err = _alloc.Call(ctx, 8)
	if err != nil {
		return fmt.Errorf("alloc: %w", err)
	}
	sizePtr := res[0]
	defer _free.Call(ctx, sizePtr)

	res, err = _encode.Call(ctx, inPtr, uint64(img.Bounds().Dx()), uint64(img.Bounds().Dy()), sizePtr, uint64(quality), uint64(qualityAlpha), uint64(speed))
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	size, ok := mod.Memory().ReadUint64Le(uint32(sizePtr))
	if !ok {
		return ErrMemRead
	}

	if size == 0 {
		return ErrEncode
	}

	defer _free.Call(ctx, res[0])

	out, ok := mod.Memory().Read(uint32(res[0]), uint32(size))
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
	mod api.Module

	_alloc  api.Function
	_free   api.Function
	_decode api.Function
	_encode api.Function

	initialized atomic.Bool
)

func initialize() {
	if initialized.Load() {
		return
	}

	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)

	r, err := gzip.NewReader(bytes.NewReader(avifWasm))
	if err != nil {
		panic(err)
	}

	var data bytes.Buffer
	_, err = data.ReadFrom(r)
	if err != nil {
		panic(err)
	}

	compiled, err := rt.CompileModule(ctx, data.Bytes())
	if err != nil {
		panic(err)
	}

	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	mod, err = rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithStderr(os.Stderr).WithStdout(os.Stdout))
	if err != nil {
		panic(err)
	}

	_alloc = mod.ExportedFunction("malloc")
	_free = mod.ExportedFunction("free")
	_decode = mod.ExportedFunction("decode")
	_encode = mod.ExportedFunction("encode")

	initialized.Store(true)
}
