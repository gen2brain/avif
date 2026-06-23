//go:build !wasm2go

package avif

import (
	"bytes"
	"compress/gzip"
	"context"
	"debug/pe"
	_ "embed"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"runtime"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed lib/avif.wasm.gz
var avifWasm []byte

func decode(r io.Reader, configOnly, decodeAll bool) (*AVIF, image.Config, error) {
	initOnce()

	var cfg image.Config

	ctx := context.Background()
	mod, err := rt.InstantiateModule(ctx, cm, mc)
	if err != nil {
		return nil, cfg, err
	}

	defer mod.Close(ctx)

	_alloc := mod.ExportedFunction("malloc")
	_free := mod.ExportedFunction("free")
	_decode := mod.ExportedFunction("decode")

	data, err := io.ReadAll(r)
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

	ok := mod.Memory().Write(uint32(inPtr), data)
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
			pix := make([]byte, size)
			for j := 0; j < size; j += 2 {
				binary.BigEndian.PutUint16(pix[j:], binary.LittleEndian.Uint16(out[j:]))
			}

			img := image.NewRGBA64(image.Rect(0, 0, cfg.Width, cfg.Height))
			img.Pix = pix
			images = append(images, img)
		} else {
			img := image.NewRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
			img.Pix = out
			images = append(images, img)
		}

		d, ok := mod.Memory().ReadUint64Le(uint32(delayPtr) + uint32(i*8))
		if !ok {
			return nil, cfg, ErrMemRead
		}

		delay = append(delay, math.Float64frombits(d))

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

func encode(w io.Writer, m image.Image, quality, qualityAlpha, speed int, subsampleRatio image.YCbCrSubsampleRatio, lossless bool) error {
	initOnce()

	ctx := context.Background()
	mod, err := rt.InstantiateModule(ctx, cm, mc)
	if err != nil {
		return err
	}

	defer mod.Close(ctx)

	_alloc := mod.ExportedFunction("malloc")
	_free := mod.ExportedFunction("free")
	_encode := mod.ExportedFunction("encode")

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

	ll := uint64(0)
	if lossless {
		ll = 1
	}

	res, err = _encode.Call(ctx, inPtr, uint64(img.Bounds().Dx()), uint64(img.Bounds().Dy()), sizePtr,
		uint64(quality), uint64(qualityAlpha), uint64(speed), uint64(chroma), ll)
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
	rt wazero.Runtime
	cm wazero.CompiledModule
	mc wazero.ModuleConfig

	initOnce = sync.OnceFunc(initialize)
)

func initialize() {
	ctx := context.Background()

	cfg := wazero.NewRuntimeConfig().WithCoreFeatures(api.CoreFeaturesV2 | experimental.CoreFeaturesThreads)
	rt = wazero.NewRuntimeWithConfig(ctx, cfg)

	r, err := gzip.NewReader(bytes.NewReader(avifWasm))
	if err != nil {
		panic(err)
	}
	defer r.Close()

	var data bytes.Buffer
	_, err = data.ReadFrom(r)
	if err != nil {
		panic(err)
	}

	cm, err = rt.CompileModule(ctx, data.Bytes())
	if err != nil {
		panic(err)
	}

	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	if runtime.GOOS == "windows" && isWindowsGUI() {
		mc = wazero.NewModuleConfig().WithStderr(io.Discard).WithStdout(io.Discard)
	} else {
		mc = wazero.NewModuleConfig().WithStderr(os.Stderr).WithStdout(os.Stdout)
	}
}

func isWindowsGUI() bool {
	const imageSubsystemWindowsGui = 2

	fileName, err := os.Executable()
	if err != nil {
		return false
	}

	fl, err := pe.Open(fileName)
	if err != nil {
		return false
	}

	defer fl.Close()

	var subsystem uint16
	if header, ok := fl.OptionalHeader.(*pe.OptionalHeader64); ok {
		subsystem = header.Subsystem
	} else if header, ok := fl.OptionalHeader.(*pe.OptionalHeader32); ok {
		subsystem = header.Subsystem
	}

	if subsystem == imageSubsystemWindowsGui {
		return true
	}

	return false
}
