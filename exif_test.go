package avif

import (
	"bytes"
	_ "embed"
	"testing"
)

//go:embed testdata/test_exif.avif
var testAvifExif []byte

func TestDecodeExif(t *testing.T) {
	ex, err := DecodeExif(bytes.NewReader(testAvifExif))
	if err != nil {
		t.Fatal(err)
	}

	if ex.Orientation != 6 {
		t.Errorf("Orientation = %d, want 6", ex.Orientation)
	}

	if ex.Make != "TestCam" {
		t.Errorf("Make = %q, want %q", ex.Make, "TestCam")
	}

	if ex.Model != "Model123" {
		t.Errorf("Model = %q, want %q", ex.Model, "Model123")
	}

	if ex.ISOSpeed != 800 {
		t.Errorf("ISOSpeed = %d, want 800", ex.ISOSpeed)
	}
}

func TestDecodeExifNone(t *testing.T) {
	_, err := DecodeExif(bytes.NewReader(testAvifRot))
	if err != ErrNoExif {
		t.Errorf("got %v, want ErrNoExif", err)
	}
}
