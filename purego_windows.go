//go:build windows

package avif

import (
	"fmt"

	"golang.org/x/sys/windows"
)

const (
	libname = "libavif.dll"
)

func loadLibrary() (uintptr, error) {
	handle, err := windows.LoadLibrary(libname)
	if err != nil {
		return 0, fmt.Errorf("cannot load library %s: %w", libname, err)
	}

	return uintptr(handle)
}
