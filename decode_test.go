package avif_test

import (
	"bytes"
	_ "embed"
	"image"
	"image/jpeg"
	"io"
	"testing"

	"github.com/gen2brain/avif"
)

//go:embed testdata/test8.avif
var testAvif8 []byte

//go:embed testdata/test10.avif
var testAvif10 []byte

//go:embed testdata/test.avifs
var testAvifAnim []byte

func TestDecode(t *testing.T) {
	img, err := avif.Decode(bytes.NewReader(testAvif8))
	if err != nil {
		t.Fatal(err)
	}

	err = jpeg.Encode(io.Discard, img, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecode10(t *testing.T) {
	img, err := avif.Decode(bytes.NewReader(testAvif10))
	if err != nil {
		t.Fatal(err)
	}

	err = jpeg.Encode(io.Discard, img, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecodeAnim(t *testing.T) {
	ret, err := avif.DecodeAll(bytes.NewReader(testAvifAnim))
	if err != nil {
		t.Fatal(err)
	}

	if len(ret.Image) != len(ret.Delay) {
		t.Errorf("got %d, want %d", len(ret.Delay), len(ret.Image))
	}

	if len(ret.Image) != 17 {
		t.Errorf("got %d, want %d", len(ret.Image), 48)
	}

	for _, img := range ret.Image {
		err = jpeg.Encode(io.Discard, img, nil)
		if err != nil {
			t.Error(err)
		}
	}
}

func TestImageDecode(t *testing.T) {
	img, _, err := image.Decode(bytes.NewReader(testAvif8))
	if err != nil {
		t.Fatal(err)
	}

	err = jpeg.Encode(io.Discard, img, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestImageDecodeAnim(t *testing.T) {
	img, _, err := image.Decode(bytes.NewReader(testAvifAnim))
	if err != nil {
		t.Fatal(err)
	}

	err = jpeg.Encode(io.Discard, img, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecodeConfig(t *testing.T) {
	cfg, err := avif.DecodeConfig(bytes.NewReader(testAvif8))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Width != 512 {
		t.Errorf("width: got %d, want %d", cfg.Width, 512)
	}

	if cfg.Height != 512 {
		t.Errorf("height: got %d, want %d", cfg.Height, 512)
	}
}

func BenchmarkDecodeJPEG(b *testing.B) {
	img, _, err := image.Decode(bytes.NewReader(testAvif8))
	if err != nil {
		b.Error(err)
	}

	var testJpeg bytes.Buffer
	err = jpeg.Encode(&testJpeg, img, nil)
	if err != nil {
		b.Error(err)
	}

	for i := 0; i < b.N; i++ {
		_, _, err := image.Decode(bytes.NewReader(testJpeg.Bytes()))
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkDecodeAVIF(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, err := image.Decode(bytes.NewReader(testAvif8))
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkDecodeConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := avif.DecodeConfig(bytes.NewReader(testAvif8))
		if err != nil {
			b.Error(err)
		}
	}
}
