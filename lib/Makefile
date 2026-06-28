LIBAVIF_VERSION = v1.4.2
LIBAVIF_AOM_VERSION = v3.14.1
LIBAVIF_DAV1D_VERSION = 1.5.3
LIBAVIF_YUV_VERSION = stable

LIBAVIF_SRC = $(PWD)/libavif
LIBAVIF_BUILD = $(LIBAVIF_SRC)/build
LIBAVIF_AOM_SRC = $(LIBAVIF_SRC)/ext/aom
LIBAVIF_AOM_BUILD = $(LIBAVIF_AOM_SRC)/build.libavif
LIBAVIF_DAV1D_SRC = $(LIBAVIF_SRC)/ext/dav1d
LIBAVIF_DAV1D_BUILD = $(LIBAVIF_DAV1D_SRC)/build
LIBAVIF_YUV_SRC = $(LIBAVIF_SRC)/ext/libyuv
LIBAVIF_YUV_BUILD = $(LIBAVIF_YUV_SRC)/build

WASI_SDK_PATH = /opt/wasi-sdk
export CC = $(WASI_SDK_PATH)/bin/clang --sysroot=$(WASI_SDK_PATH)/share/wasi-sysroot --target=wasm32-wasip1

CMAKE_TOOLCHAIN_FILE = $(PWD)/toolchain.cmake
MESON_TOOLCHAIN_FILE = $(PWD)/wasi.meson

BIN := avif.wasm

all: $(BIN) wasm2go

wasm2go: ../libavif.go

../libavif.go: $(BIN)
	wasm2go -pkg avif -unsafe -o ../libavif.go $(BIN)
	sed -i -E '/^const data[0-9]/! { s/\bModule\b/module/g; s/\bMemory\b/memory/g; s/\bNew\b/newModuleRaw/g; s/\bXwasi_snapshot_preview1\b/wasiSnapshotPreview1/g; }' ../libavif.go

$(LIBAVIF_SRC):
	git clone -b $(LIBAVIF_VERSION) --depth 1 https://github.com/AOMediaCodec/libavif libavif
	mkdir -p $(LIBAVIF_BUILD)
	test -d $@

$(LIBAVIF_AOM_SRC): $(LIBAVIF_SRC)
	cd $(LIBAVIF_SRC)/ext; \
	git clone -b $(LIBAVIF_AOM_VERSION) --depth 1 --recursive --jobs `nproc` https://aomedia.googlesource.com/aom
	mkdir -p $(LIBAVIF_AOM_BUILD)
	test -d $@

$(LIBAVIF_DAV1D_SRC): $(LIBAVIF_SRC)
	cd $(LIBAVIF_SRC)/ext; \
	git clone -b $(LIBAVIF_DAV1D_VERSION) --depth 1 https://code.videolan.org/videolan/dav1d.git dav1d
	sed -i "s/'emscripten'/'emscripten', 'wasi'/g" $(LIBAVIF_DAV1D_SRC)/meson.build
	mkdir -p $(LIBAVIF_DAV1D_BUILD)
	test -d $@

$(LIBAVIF_YUV_SRC): $(LIBAVIF_SRC)
	cd $(LIBAVIF_SRC)/ext; \
	git clone -b $(LIBAVIF_YUV_VERSION) --depth 1 https://chromium.googlesource.com/libyuv/libyuv
	sed -i '/^ADD_EXECUTABLE\|^TARGET_LINK_LIBRARIES/d' $(LIBAVIF_YUV_SRC)/CMakeLists.txt
	mkdir -p $(LIBAVIF_YUV_BUILD)
	test -d $@

$(LIBAVIF_AOM_BUILD)/libaom.a: $(LIBAVIF_AOM_SRC)
	cd $(LIBAVIF_AOM_BUILD); \
	cmake $(LIBAVIF_AOM_SRC) \
		-DCMAKE_BUILD_TYPE=MinSizeRel \
		-DBUILD_SHARED_LIBS=0 \
		-DENABLE_DOCS=0 \
		-DENABLE_EXAMPLES=0 \
		-DENABLE_TESTDATA=0 \
		-DENABLE_TESTS=0 \
		-DENABLE_TOOLS=0 \
		-DAOM_TARGET_CPU=generic \
		-DCONFIG_RUNTIME_CPU_DETECT=0 \
		-DCONFIG_MULTITHREAD=0 \
		-DCONFIG_WEBM_IO=0 \
		-DCONFIG_AV1_DECODER=0 \
		-DCONFIG_AV1_ENCODER=1 \
		-DCMAKE_TOOLCHAIN_FILE=$(CMAKE_TOOLCHAIN_FILE)

	cd $(LIBAVIF_AOM_BUILD); \
	$(MAKE) -j$(shell nproc)

$(LIBAVIF_DAV1D_BUILD)/src/libdav1d.a: $(LIBAVIF_DAV1D_SRC)
	cd $(LIBAVIF_DAV1D_BUILD); \
	meson setup \
		--default-library=static \
		--buildtype=release \
		--cross-file=$(MESON_TOOLCHAIN_FILE) \
		-Denable_tools=false \
		-Denable_tests=false \
		-Denable_asm=false \
		..

	cd $(LIBAVIF_DAV1D_BUILD); \
	ninja -j$(shell nproc)

$(LIBAVIF_YUV_BUILD)/libyuv.a: $(LIBAVIF_YUV_SRC)
	cd $(LIBAVIF_YUV_BUILD); \
	cmake $(LIBAVIF_YUV_SRC) \
		-DCMAKE_BUILD_TYPE=MinSizeRel \
		-DCMAKE_POLICY_VERSION_MINIMUM=3.5 \
		-DBUILD_SHARED_LIBS=0 \
		-DCMAKE_TOOLCHAIN_FILE=$(CMAKE_TOOLCHAIN_FILE)

	cd $(LIBAVIF_YUV_BUILD); \
	$(MAKE) -j$(shell nproc)

$(LIBAVIF_BUILD)/libavif.a: $(LIBAVIF_AOM_BUILD)/libaom.a $(LIBAVIF_DAV1D_BUILD)/src/libdav1d.a $(LIBAVIF_YUV_BUILD)/libyuv.a
	cd $(LIBAVIF_BUILD); \
	cmake $(LIBAVIF_SRC) \
		-DCMAKE_BUILD_TYPE=MinSizeRel \
		-DBUILD_SHARED_LIBS=0 \
		-DAVIF_CODEC_DAV1D=LOCAL \
		-DAVIF_CODEC_AOM=LOCAL \
		-DAVIF_CODEC_AOM_DECODE=0 \
		-DAVIF_CODEC_AOM_ENCODE=1 \
		-DAVIF_LIBYUV=LOCAL \
		-DCMAKE_TOOLCHAIN_FILE=$(CMAKE_TOOLCHAIN_FILE)

	cd $(LIBAVIF_BUILD); \
	$(MAKE) -j$(shell nproc)

$(BIN): $(LIBAVIF_BUILD)/libavif.a
	$(CC) \
		-O3 \
		-Wl,--no-entry \
		-Wl,--export=malloc \
		-Wl,--export=free \
		-Wl,--export=decode \
		-Wl,--export=encode \
		-mexec-model=reactor \
		-mnontrapping-fptoint \
		-z stack-size=1048576 \
		-I $(LIBAVIF_SRC)/include \
		-o $@ \
		-Wall \
		avif.c \
		${LIBAVIF_BUILD}/libavif.a \
		${LIBAVIF_YUV_BUILD}/libyuv.a \
		${LIBAVIF_DAV1D_BUILD}/src/libdav1d.a \
		${LIBAVIF_AOM_BUILD}/libaom.a

.PHONY: all wasm2go clean

clean:
	-rm -rf $(LIBAVIF_SRC)
