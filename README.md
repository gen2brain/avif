## avif
[![Status](https://github.com/gen2brain/avif/actions/workflows/test.yml/badge.svg)](https://github.com/gen2brain/avif/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/gen2brain/avif.svg)](https://pkg.go.dev/github.com/gen2brain/avif)

Go encoder/decoder for [AV1 Image File Format (AVIF)](https://en.wikipedia.org/wiki/AVIF) with support for animated AVIF images (decode only).

Based on [libavif](https://github.com/AOMediaCodec/libavif) and [aom](https://aomedia.googlesource.com/aom/) compiled to [WASM](https://en.wikipedia.org/wiki/WebAssembly) and used with [wazero](https://wazero.io/) runtime (CGo-free).

The library will first try to use a dynamic/shared library (if installed) via [purego](https://github.com/ebitengine/purego) and will fall back to WASM.
