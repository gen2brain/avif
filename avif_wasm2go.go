package avif

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"runtime"
	"unsafe"
)

func decode(r io.Reader, configOnly, decodeAll bool) (ret *AVIF, cfg image.Config, err error) {
	mod := newModule()

	defer func() {
		if e := recover(); e != nil {
			if _, ok := e.(procExit); ok {
				ret, err = nil, ErrDecode
				return
			}
			panic(e)
		}
	}()

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, cfg, fmt.Errorf("read: %w", err)
	}

	inSize := len(data)

	inPtr := mod.Xmalloc(int32(inSize))
	defer mod.Xfree(inPtr)

	ok := mod.write(inPtr, data)
	if !ok {
		return nil, cfg, ErrMemWrite
	}

	ptr := mod.Xmalloc(4 * 4)
	defer mod.Xfree(ptr)

	widthPtr := ptr
	heightPtr := ptr + 4
	depthPtr := ptr + 8
	countPtr := ptr + 12

	res := mod.Xdecode(inPtr, int32(inSize), 1, 0, widthPtr, heightPtr, depthPtr, countPtr, 0, 0)
	if res == 0 {
		return nil, cfg, ErrDecode
	}

	width, ok := mod.readUint32(widthPtr)
	if !ok {
		return nil, cfg, ErrMemRead
	}

	height, ok := mod.readUint32(heightPtr)
	if !ok {
		return nil, cfg, ErrMemRead
	}

	depth, ok := mod.readUint32(depthPtr)
	if !ok {
		return nil, cfg, ErrMemRead
	}

	count, ok := mod.readUint32(countPtr)
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

	outPtr := mod.Xmalloc(int32(outSize))
	defer mod.Xfree(outPtr)

	delaySize := 8
	if decodeAll {
		delaySize = 8 * int(count)
	}

	delayPtr := mod.Xmalloc(int32(delaySize))
	defer mod.Xfree(delayPtr)

	all := int32(0)
	if decodeAll {
		all = 1
	}

	res = mod.Xdecode(inPtr, int32(inSize), 0, all, widthPtr, heightPtr, depthPtr, countPtr, delayPtr, outPtr)
	if res == 0 {
		return nil, cfg, ErrDecode
	}

	delay := make([]float64, 0)
	images := make([]image.Image, 0)

	for i := 0; i < int(count); i++ {
		out, ok := mod.read(outPtr+int32(i*size), int32(size))
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

		d, ok := mod.readFloat64(delayPtr + int32(i*8))
		if !ok {
			return nil, cfg, ErrMemRead
		}

		delay = append(delay, d)

		if !decodeAll {
			break
		}
	}

	ret = &AVIF{
		Image: images,
		Delay: delay,
	}

	return ret, cfg, nil
}

func encode(w io.Writer, m image.Image, quality, qualityAlpha, speed int, subsampleRatio image.YCbCrSubsampleRatio) (err error) {
	mod := newModule()

	defer func() {
		if e := recover(); e != nil {
			if _, ok := e.(procExit); ok {
				err = ErrEncode
				return
			}
			panic(e)
		}
	}()

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

	inPtr := mod.Xmalloc(int32(len(img.Pix)))
	defer mod.Xfree(inPtr)

	ok := mod.write(inPtr, img.Pix)
	if !ok {
		return ErrMemWrite
	}

	sizePtr := mod.Xmalloc(8)
	defer mod.Xfree(sizePtr)

	outPtr := mod.Xencode(inPtr, int32(img.Bounds().Dx()), int32(img.Bounds().Dy()), sizePtr, int32(quality),
		int32(qualityAlpha), int32(speed), int32(chroma))

	size, ok := mod.readUint64(sizePtr)
	if !ok {
		return ErrMemRead
	}

	if size == 0 {
		return ErrEncode
	}

	defer mod.Xfree(outPtr)

	out, ok := mod.read(outPtr, int32(size))
	if !ok {
		return ErrMemRead
	}

	_, err = w.Write(out)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func newModule() *Module {
	stdout, stderr := stdWriters()
	mod := New(&wasiHost{stdout: stdout, stderr: stderr})
	mod.X_initialize()

	return mod
}

func (m *Module) write(ptr int32, data []byte) bool {
	if ptr < 0 || int(ptr)+len(data) > len(m.memory) {
		return false
	}

	copy(m.memory[ptr:], data)

	return true
}

func (m *Module) read(ptr, size int32) ([]byte, bool) {
	if ptr < 0 || size < 0 || int(ptr)+int(size) > len(m.memory) {
		return nil, false
	}

	return m.memory[ptr : ptr+size : ptr+size], true
}

func (m *Module) readUint32(ptr int32) (uint32, bool) {
	if ptr < 0 || int(ptr)+4 > len(m.memory) {
		return 0, false
	}

	return load32(m.memory[ptr:]), true
}

func (m *Module) readUint64(ptr int32) (uint64, bool) {
	if ptr < 0 || int(ptr)+8 > len(m.memory) {
		return 0, false
	}

	return load64(m.memory[ptr:]), true
}

func (m *Module) readFloat64(ptr int32) (float64, bool) {
	v, ok := m.readUint64(ptr)
	if !ok {
		return 0, false
	}

	return math.Float64frombits(v), true
}

// procExit carries the exit code of a wasi proc_exit call so Decode/Encode can
// turn an unexpected module abort into an error instead of crashing.
type procExit struct {
	code int32
}

// errBadf is the wasi EBADF errno, returned from the unused file-I/O imports.
const errBadf = 8

// wasiHost implements the wasi_snapshot_preview1 imports the avif module links
// against. dav1d/aom only emit diagnostic output (fd_write) and may abort
// (proc_exit); the file-I/O calls are never reached and return EBADF.
type wasiHost struct {
	mod    *Module
	stdout io.Writer
	stderr io.Writer
}

func (h *wasiHost) Init(m any) {
	h.mod = m.(*Module)
}

func (h *wasiHost) Xclock_time_get(id int32, precision int64, retPtr int32) int32 {
	store64(h.mod.memory[retPtr:], 0)
	return 0
}

func (h *wasiHost) Xfd_close(fd int32) int32 {
	return 0
}

func (h *wasiHost) Xfd_fdstat_get(fd, retPtr int32) int32 {
	return errBadf
}

func (h *wasiHost) Xfd_fdstat_set_flags(fd, flags int32) int32 {
	return errBadf
}

func (h *wasiHost) Xfd_prestat_dir_name(fd, pathPtr, pathLen int32) int32 {
	return errBadf
}

func (h *wasiHost) Xfd_prestat_get(fd, retPtr int32) int32 {
	return errBadf
}

func (h *wasiHost) Xfd_read(fd, iovs, iovsLen, nreadPtr int32) int32 {
	return errBadf
}

func (h *wasiHost) Xfd_seek(fd int32, offset int64, whence, retPtr int32) int32 {
	return errBadf
}

func (h *wasiHost) Xpath_open(dirFd, dirFlags, pathPtr, pathLen, oflags int32, rights, rightsInheriting int64, fdFlags, fdPtr int32) int32 {
	return errBadf
}

func (h *wasiHost) Xproc_exit(code int32) {
	panic(procExit{code})
}

func (h *wasiHost) Xfd_write(fd, iovs, iovsLen, nwrittenPtr int32) int32 {
	mem := h.mod.memory

	var dst io.Writer
	switch fd {
	case 1:
		dst = h.stdout
	case 2:
		dst = h.stderr
	default:
		dst = io.Discard
	}

	var written uint32
	for i := int32(0); i < iovsLen; i++ {
		ptr := load32(mem[iovs+i*8:])
		length := load32(mem[iovs+i*8+4:])
		if length != 0 {
			dst.Write(mem[ptr : ptr+length])
		}
		written += length
	}

	store32(mem[nwrittenPtr:], written)

	return 0
}

func stdWriters() (io.Writer, io.Writer) {
	if runtime.GOOS == "windows" && isWindowsGUI() {
		return io.Discard, io.Discard
	}

	return os.Stdout, os.Stderr
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
