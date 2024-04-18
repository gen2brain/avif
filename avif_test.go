package avif

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"testing"
)

//go:embed testdata/test8.avif
var testAvif8 []byte

//go:embed testdata/test10.avif
var testAvif10 []byte

//go:embed testdata/test.avifs
var testAvifAnim []byte

func TestDecode(t *testing.T) {
	img, err := Decode(bytes.NewReader(testAvif8))
	if err != nil {
		t.Fatal(err)
	}

	err = jpeg.Encode(io.Discard, img, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecode10(t *testing.T) {
	img, err := Decode(bytes.NewReader(testAvif10))
	if err != nil {
		t.Fatal(err)
	}

	err = jpeg.Encode(io.Discard, img, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecodeWASM(t *testing.T) {
	img, _, err := decode(bytes.NewReader(testAvif8), false, false)
	if err != nil {
		t.Fatal(err)
	}

	err = jpeg.Encode(io.Discard, img.Image[0], nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecodeDynamic(t *testing.T) {
	if err := Dynamic(); err != nil {
		fmt.Println(err)
		t.Skip()
	}

	img, _, err := decodeDynamic(bytes.NewReader(testAvif8), false, false)
	if err != nil {
		t.Fatal(err)
	}

	err = jpeg.Encode(io.Discard, img.Image[0], nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecodeAnim(t *testing.T) {
	ret, err := DecodeAll(bytes.NewReader(testAvifAnim))
	if err != nil {
		t.Fatal(err)
	}

	if len(ret.Image) != len(ret.Delay) {
		t.Errorf("got %d, want %d", len(ret.Delay), len(ret.Image))
	}

	if len(ret.Image) != 17 {
		t.Errorf("got %d, want %d", len(ret.Image), 17)
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
	cfg, err := DecodeConfig(bytes.NewReader(testAvif8))
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

func TestEncode(t *testing.T) {
	img, err := Decode(bytes.NewReader(testAvif8))
	if err != nil {
		t.Fatal(err)
	}

	err = Encode(io.Discard, img)
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkDecode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, err := decode(bytes.NewReader(testAvif8), false, false)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkDecodeDynamic(b *testing.B) {
	if err := Dynamic(); err != nil {
		fmt.Println(err)
		b.Skip()
	}

	for i := 0; i < b.N; i++ {
		_, _, err := decodeDynamic(bytes.NewReader(testAvif8), false, false)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkDecodeConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, err := decode(bytes.NewReader(testAvif8), true, false)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkDecodeConfigDynamic(b *testing.B) {
	if err := Dynamic(); err != nil {
		fmt.Println(err)
		b.Skip()
	}

	for i := 0; i < b.N; i++ {
		_, _, err := decodeDynamic(bytes.NewReader(testAvif8), true, false)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkEncode(b *testing.B) {
	img, err := Decode(bytes.NewReader(testAvif8))
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		err := encode(io.Discard, img, DefaultQuality, DefaultQuality, DefaultSpeed)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkEncodeDynamic(b *testing.B) {
	if err := Dynamic(); err != nil {
		fmt.Println(err)
		b.Skip()
	}

	img, err := Decode(bytes.NewReader(testAvif8))
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		err := encodeDynamic(io.Discard, img, DefaultQuality, DefaultQuality, DefaultSpeed)
		if err != nil {
			b.Error(err)
		}
	}
}
