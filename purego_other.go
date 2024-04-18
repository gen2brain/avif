//go:build !unix && !darwin && !windows

package avif

import (
	"fmt"
	"image"
	"io"
	"runtime"
)

var (
	dynamic    = false
	dynamicErr = fmt.Errorf("avif: unsupported os: %s", runtime.GOOS)
)

func decodeDynamic(r io.Reader, configOnly, decodeAll bool) (*AVIF, image.Config, error) {
	return nil, image.Config{}, dynamicErr
}

func encodeDynamic(w io.Writer, m image.Image, quality, qualityAlpha, speed int) error {
	return dynamicErr
}

func loadLibrary() (uintptr, error) {
	return 0, dynamicErr
}
