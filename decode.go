package avif

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"sync/atomic"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/emscripten"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed lib/avif.wasm
var avifWasm []byte

// Errors .
var (
	ErrMemRead  = errors.New("avif: mem read failed")
	ErrMemWrite = errors.New("avif: mem write failed")
	ErrDecode   = errors.New("avif: decode failed")
)

// Decode reads a AVIF image from r and returns it as an image.Image.
func Decode(r io.Reader) (image.Image, error) {
	imgs, _, _, err := decode(r, false, false)
	if err != nil {
		return nil, err
	}

	return imgs[0], nil
}

// DecodeConfig returns the color model and dimensions of a AVIF image without decoding the entire image.
func DecodeConfig(r io.Reader) (image.Config, error) {
	_, cfg, _, err := decode(r, true, false)
	if err != nil {
		return image.Config{}, err
	}

	return cfg, nil
}

// DecodeAll reads a AVIF image from r and returns the sequential frames and timing information.
func DecodeAll(r io.Reader) ([]image.Image, []float64, error) {
	imgs, _, delay, err := decode(r, false, true)
	if err != nil {
		return nil, nil, err
	}

	return imgs, delay, nil
}

func decode(r io.Reader, configOnly, decodeAll bool) ([]image.Image, image.Config, []float64, error) {
	if !initialized.Load() {
		initialize()
	}

	var cfg image.Config
	var avif bytes.Buffer

	_, err := avif.ReadFrom(r)
	if err != nil {
		return nil, cfg, nil, fmt.Errorf("read: %w", err)
	}

	inSize := avif.Len()
	ctx := context.Background()

	res, err := _alloc.Call(ctx, uint64(inSize))
	if err != nil {
		return nil, cfg, nil, fmt.Errorf("alloc: %w", err)
	}
	inPtr := res[0]
	defer _free.Call(ctx, inPtr)

	ok := mod.Memory().Write(uint32(inPtr), avif.Bytes())
	if !ok {
		return nil, cfg, nil, ErrMemWrite
	}

	res, err = _alloc.Call(ctx, 4)
	if err != nil {
		return nil, cfg, nil, fmt.Errorf("alloc: %w", err)
	}
	widthPtr := res[0]
	defer _free.Call(ctx, widthPtr)

	res, err = _alloc.Call(ctx, 4)
	if err != nil {
		return nil, cfg, nil, fmt.Errorf("alloc: %w", err)
	}
	heightPtr := res[0]
	defer _free.Call(ctx, heightPtr)

	res, err = _alloc.Call(ctx, 4)
	if err != nil {
		return nil, cfg, nil, fmt.Errorf("alloc: %w", err)
	}
	depthPtr := res[0]
	defer _free.Call(ctx, depthPtr)

	res, err = _alloc.Call(ctx, 4)
	if err != nil {
		return nil, cfg, nil, fmt.Errorf("alloc: %w", err)
	}
	countPtr := res[0]
	defer _free.Call(ctx, countPtr)

	res, err = _decode.Call(ctx, inPtr, uint64(inSize), 1, 0, widthPtr, heightPtr, depthPtr, countPtr, 0, 0)
	if err != nil {
		return nil, cfg, nil, fmt.Errorf("decode: %w", err)
	}

	if res[0] == 0 {
		return nil, cfg, nil, ErrDecode
	}

	width, ok := mod.Memory().ReadUint32Le(uint32(widthPtr))
	if !ok {
		return nil, cfg, nil, ErrMemRead
	}

	height, ok := mod.Memory().ReadUint32Le(uint32(heightPtr))
	if !ok {
		return nil, cfg, nil, ErrMemRead
	}

	depth, ok := mod.Memory().ReadUint32Le(uint32(depthPtr))
	if !ok {
		return nil, cfg, nil, ErrMemRead
	}

	count, ok := mod.Memory().ReadUint32Le(uint32(countPtr))
	if !ok {
		return nil, cfg, nil, ErrMemRead
	}

	cfg.Width = int(width)
	cfg.Height = int(height)

	cfg.ColorModel = color.NRGBAModel
	if depth > 8 {
		cfg.ColorModel = color.NRGBA64Model
	}

	if configOnly {
		return nil, cfg, nil, nil
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
		return nil, cfg, nil, fmt.Errorf("alloc: %w", err)
	}
	outPtr := res[0]
	defer _free.Call(ctx, outPtr)

	delaySize := 8
	if decodeAll {
		delaySize = 8 * int(count)
	}

	res, err = _alloc.Call(ctx, uint64(delaySize))
	if err != nil {
		return nil, cfg, nil, fmt.Errorf("alloc: %w", err)
	}
	delayPtr := res[0]
	defer _free.Call(ctx, delayPtr)

	all := 0
	if decodeAll {
		all = 1
	}

	res, err = _decode.Call(ctx, inPtr, uint64(inSize), 0, uint64(all), widthPtr, heightPtr, depthPtr, countPtr, delayPtr, outPtr)
	if err != nil {
		return nil, cfg, nil, fmt.Errorf("decode: %w", err)
	}

	if res[0] == 0 {
		return nil, cfg, nil, ErrDecode
	}

	delay := make([]float64, 0)
	images := make([]image.Image, 0)

	for i := 0; i < int(count); i++ {
		out, ok := mod.Memory().Read(uint32(outPtr)+uint32(i*size), uint32(size))
		if !ok {
			return nil, cfg, nil, ErrMemRead
		}

		if depth > 8 {
			img := image.NewNRGBA64(image.Rect(0, 0, cfg.Width, cfg.Height))
			img.Pix = out
			images = append(images, img)
		} else {
			img := image.NewNRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
			img.Pix = out
			images = append(images, img)
		}

		d, ok := mod.Memory().ReadFloat64Le(uint32(delayPtr) + uint32(i*8))
		if !ok {
			return nil, cfg, nil, ErrMemRead
		}

		delay = append(delay, d)

		if !decodeAll {
			break
		}
	}

	return images, cfg, delay, nil
}

var (
	mod api.Module

	_alloc  api.Function
	_free   api.Function
	_decode api.Function

	initialized atomic.Bool
)

func initialize() {
	if initialized.Load() {
		return
	}

	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)

	compiled, err := rt.CompileModule(ctx, avifWasm)
	if err != nil {
		panic(err)
	}

	_, err = emscripten.InstantiateForModule(ctx, rt, compiled)
	if err != nil {
		panic(err)
	}

	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	mod, err = rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithStderr(os.Stderr).WithStdout(os.Stdout))
	if err != nil {
		panic(err)
	}

	_alloc = mod.ExportedFunction("allocate")
	_free = mod.ExportedFunction("deallocate")
	_decode = mod.ExportedFunction("decode")

	initialized.Store(true)
}
