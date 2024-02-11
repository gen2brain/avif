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

var ErrMemRead = errors.New("mem read failed")
var ErrMemWrite = errors.New("mem write failed")

// Decode reads a AVIF image from r and returns it as an image.Image.
func Decode(r io.Reader) (image.Image, error) {
	img, _, err := decode(r, false)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// DecodeConfig returns the color model and dimensions of a AVIF image without decoding the entire image.
func DecodeConfig(r io.Reader) (image.Config, error) {
	_, cfg, err := decode(r, true)
	if err != nil {
		return image.Config{}, err
	}

	return cfg, nil
}

func decode(r io.Reader, configOnly bool) (image.Image, image.Config, error) {
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

	var cfgOnly uint64
	if configOnly {
		cfgOnly = 1
	}

	res, err = _decode.Call(ctx, inPtr, uint64(inSize), cfgOnly, widthPtr, heightPtr)
	if err != nil {
		return nil, cfg, fmt.Errorf("decode: %w", err)
	}
	imagePtr := res[0]
	defer _free.Call(ctx, imagePtr)

	width, ok := mod.Memory().ReadUint32Le(uint32(widthPtr))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	height, ok := mod.Memory().ReadUint32Le(uint32(heightPtr))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	cfg.Width = int(width)
	cfg.Height = int(height)
	cfg.ColorModel = color.RGBAModel

	if configOnly {
		return nil, cfg, nil
	}

	size := cfg.Width * cfg.Height * 4

	res, err = _alloc.Call(ctx, uint64(size))
	if err != nil {
		return nil, cfg, fmt.Errorf("alloc: %w", err)
	}
	outPtr := res[0]
	defer _free.Call(ctx, outPtr)

	_, err = _rgba.Call(ctx, imagePtr, outPtr)
	if err != nil {
		return nil, cfg, fmt.Errorf("rgba: %w", err)
	}

	tmp, ok := mod.Memory().Read(uint32(outPtr), uint32(size))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	img := image.NewRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
	copy(img.Pix, tmp)

	return img, cfg, nil
}

var (
	mod api.Module

	_alloc  api.Function
	_free   api.Function
	_rgba   api.Function
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
	_rgba = mod.ExportedFunction("rgba")
	_decode = mod.ExportedFunction("decode")

	initialized.Store(true)
}
