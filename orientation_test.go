package avif

import (
	"bytes"
	_ "embed"
	"image"
	"testing"
)

//go:embed testdata/test_rot.avif
var testAvifRot []byte

func TestParseProps(t *testing.T) {
	p, ok := parseAVIFProps(testAvif8)
	if !ok {
		t.Fatal("no dimensions parsed")
	}

	if p.width != 512 || p.height != 512 {
		t.Errorf("dims: got %dx%d, want 512x512", p.width, p.height)
	}

	if p.orientation != 1 {
		t.Errorf("orientation: got %d, want 1", p.orientation)
	}
}

func TestParsePropsDepth(t *testing.T) {
	p, ok := parseAVIFProps(testAvif10)
	if !ok {
		t.Fatal("no dimensions parsed")
	}

	if !p.hiDepth {
		t.Error("expected hiDepth for 10-bit image")
	}
}

func TestConfigStream(t *testing.T) {
	cfg, err := DecodeConfig(bytes.NewReader(testAvif8))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Width != 512 || cfg.Height != 512 {
		t.Errorf("got %dx%d, want 512x512", cfg.Width, cfg.Height)
	}
}

func TestConfigStreamRotated(t *testing.T) {
	cfg, err := DecodeConfig(bytes.NewReader(testAvifRot))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Width != 640 || cfg.Height != 480 {
		t.Errorf("stored dims: got %dx%d, want 640x480", cfg.Width, cfg.Height)
	}
}

func TestAutoRotate(t *testing.T) {
	p, ok := parseAVIFProps(testAvifRot)
	if !ok || p.orientation == 1 {
		t.Fatalf("expected a transform, got orientation %d", p.orientation)
	}

	plain, err := DecodeAll(bytes.NewReader(testAvifRot))
	if err != nil {
		t.Fatal(err)
	}

	pb := plain.Image[0].Bounds()
	if pb.Dx() != 640 || pb.Dy() != 480 {
		t.Errorf("plain: got %dx%d, want 640x480", pb.Dx(), pb.Dy())
	}

	rot, err := DecodeAll(bytes.NewReader(testAvifRot), Options{AutoRotate: true})
	if err != nil {
		t.Fatal(err)
	}

	rb := rot.Image[0].Bounds()
	if rb.Dx() != 480 || rb.Dy() != 640 {
		t.Errorf("rotated: got %dx%d, want 480x640", rb.Dx(), rb.Dy())
	}
}

func TestExifOrientationTable(t *testing.T) {
	cases := []struct {
		rot   bool
		angle int
		mir   bool
		axis  int
		want  int
	}{
		{false, 0, false, 0, 1},
		{true, 1, false, 0, 8},
		{true, 2, false, 0, 3},
		{true, 3, false, 0, 6},
		{false, 0, true, 1, 2},
		{false, 0, true, 0, 4},
		{true, 1, true, 1, 7},
		{true, 1, true, 0, 5},
		{true, 2, true, 1, 4},
		{true, 2, true, 0, 2},
		{true, 3, true, 1, 5},
		{true, 3, true, 0, 7},
	}

	for _, c := range cases {
		got := exifOrientationFromIrotImir(c.rot, c.angle, c.mir, c.axis)
		if got != c.want {
			t.Errorf("rot=%v angle=%d mir=%v axis=%d: got %d, want %d", c.rot, c.angle, c.mir, c.axis, got, c.want)
		}
	}
}

func TestOrientDims(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))

	for _, o := range []int{5, 6, 7, 8} {
		out := applyOrientation(src, o)
		b := out.Bounds()
		if b.Dx() != 2 || b.Dy() != 4 {
			t.Errorf("orientation %d: got %dx%d, want 2x4", o, b.Dx(), b.Dy())
		}
	}

	for _, o := range []int{2, 3, 4} {
		out := applyOrientation(src, o)
		b := out.Bounds()
		if b.Dx() != 4 || b.Dy() != 2 {
			t.Errorf("orientation %d: got %dx%d, want 4x2", o, b.Dx(), b.Dy())
		}
	}
}
