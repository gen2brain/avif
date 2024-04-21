//go:build (!unix && !darwin && !windows) || nodynamic

package avif

import (
	"fmt"
	"image"
	"io"
)

var (
	dynamic    = false
	dynamicErr = fmt.Errorf("avif: dynamic disabled")
)

func decodeDynamic(r io.Reader, configOnly, decodeAll bool) (*AVIF, image.Config, error) {
	return nil, image.Config{}, dynamicErr
}

func encodeDynamic(w io.Writer, m image.Image, quality, qualityAlpha, speed int, subsampleRatio image.YCbCrSubsampleRatio) error {
	return dynamicErr
}

func loadLibrary() (uintptr, error) {
	return 0, dynamicErr
}
