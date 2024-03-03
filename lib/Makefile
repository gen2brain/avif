LIBAVIF_VERSION = v1.0.4
LIBAVIF_DAV1D_VERSION = master
LIBAVIF_YUV_VERSION = stable

LIBAVIF_SRC = $(PWD)/libavif
LIBAVIF_BUILD = $(LIBAVIF_SRC)/build
LIBAVIF_DAV1D_SRC = $(LIBAVIF_SRC)/ext/dav1d
LIBAVIF_DAV1D_BUILD = $(LIBAVIF_DAV1D_SRC)/build
LIBAVIF_YUV_SRC = $(LIBAVIF_SRC)/ext/libyuv
LIBAVIF_YUV_BUILD = $(LIBAVIF_YUV_SRC)/build

CMAKE_TOOLCHAIN_FILE = $(EMSDK)/upstream/emscripten/cmake/Modules/Platform/Emscripten.cmake

BIN := avif.wasm

EMCMAKE := emcmake cmake
EMMAKE := emmake $(MAKE)
EMCC := emcc
EMCXX := em++

export CC = $(EMCC)
export CXX = $(EMCXX)
export CFLAGS = -O3

all: avif.wasm

$(LIBAVIF_SRC):
	git clone -b $(LIBAVIF_VERSION) --depth 1 https://github.com/AOMediaCodec/libavif
	mkdir -p $(LIBAVIF_BUILD)
	test -d $@

$(LIBAVIF_DAV1D_SRC): $(LIBAVIF_SRC)
	cd $(LIBAVIF_SRC)/ext; \
	git clone -b $(LIBAVIF_DAV1D_VERSION) --depth 1 --recursive --jobs `nproc` https://github.com/MikuAuahDark/dav1d-cmake dav1d
	mkdir -p $(LIBAVIF_DAV1D_BUILD)
	test -d $@

$(LIBAVIF_YUV_SRC): $(LIBAVIF_SRC)
	cd $(LIBAVIF_SRC)/ext; \
	git clone -b $(LIBAVIF_YUV_VERSION) --depth 1 https://chromium.googlesource.com/libyuv/libyuv
	sed -i '/^ADD_EXECUTABLE\|^TARGET_LINK_LIBRARIES/d' $(LIBAVIF_YUV_SRC)/CMakeLists.txt
	mkdir -p $(LIBAVIF_YUV_BUILD)
	test -d $@

$(LIBAVIF_DAV1D_BUILD)/libdav1d.a: $(LIBAVIF_DAV1D_SRC)
	cd $(LIBAVIF_DAV1D_BUILD); \
	$(EMCMAKE) $(LIBAVIF_DAV1D_SRC) \
		-DCMAKE_BUILD_TYPE=Release \
		-DBUILD_SHARED_LIBS=0 \
		-Denable_tools=0 \
		-Dbitdepth_16=1 \
		-DCMAKE_TOOLCHAIN_FILE=$(CMAKE_TOOLCHAIN_FILE)

	cd $(LIBAVIF_DAV1D_BUILD); \
	$(EMMAKE) -j$(shell nproc)

	mkdir -p $(LIBAVIF_DAV1D_BUILD)/src && cp $(LIBAVIF_DAV1D_BUILD)/libdav1d.a $(LIBAVIF_DAV1D_BUILD)/src/
	ln -sf $(LIBAVIF_DAV1D_SRC)/dav1d/include $(LIBAVIF_DAV1D_SRC)/include

$(LIBAVIF_YUV_BUILD)/libyuv.a: $(LIBAVIF_YUV_SRC)
	cd $(LIBAVIF_YUV_BUILD); \
	$(EMCMAKE) $(LIBAVIF_YUV_SRC) \
		-DCMAKE_BUILD_TYPE=Release \
		-DBUILD_SHARED_LIBS=0 \
		-Wno-dev \
		-DCMAKE_TOOLCHAIN_FILE=$(CMAKE_TOOLCHAIN_FILE)

	cd $(LIBAVIF_YUV_BUILD); \
	$(EMMAKE) -j$(shell nproc)

$(LIBAVIF_BUILD)/libavif.a: $(LIBAVIF_DAV1D_BUILD)/libdav1d.a $(LIBAVIF_YUV_BUILD)/libyuv.a
	cd $(LIBAVIF_BUILD); \
	$(EMCMAKE) $(LIBAVIF_SRC) \
		-DCMAKE_BUILD_TYPE=Release \
		-DBUILD_SHARED_LIBS=0 \
		-DAVIF_CODEC_DAV1D=1 \
		-DAVIF_LOCAL_DAV1D=1 \
		-DAVIF_LOCAL_LIBYUV=1 \
		-DCMAKE_TOOLCHAIN_FILE=$(CMAKE_TOOLCHAIN_FILE)

	cd $(LIBAVIF_BUILD); \
	$(EMMAKE) -j$(shell nproc)

$(BIN): $(LIBAVIF_BUILD)/libavif.a
	$(EMCC) \
		-O3 \
		--no-entry \
		-mnontrapping-fptoint \
		-sFILESYSTEM=0 \
		-sALLOW_MEMORY_GROWTH=1 \
		-sSTACK_SIZE=1MB \
		-sSTANDALONE_WASM=1 \
		-sUSE_PTHREADS=0 \
		-I libavif/include \
		-o $@ \
		-Wall \
		avif.c \
		${LIBAVIF_BUILD}/libavif.a \
		${LIBAVIF_YUV_BUILD}/libyuv.a \
		${LIBAVIF_DAV1D_BUILD}/libdav1d.a \
		${LIBAVIF_DAV1D_BUILD}/libdav1d_bitdepth_8.a \
		${LIBAVIF_DAV1D_BUILD}/libdav1d_bitdepth_16.a

.PHONY: clean

clean:
	-rm -rf $(LIBAVIF_SRC)
