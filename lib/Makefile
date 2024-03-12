LIBAVIF_VERSION = v1.0.4
LIBAVIF_DAV1D_VERSION = master
LIBAVIF_AOM_VERSION = v3.8.1
LIBAVIF_YUV_VERSION = stable

LIBAVIF_SRC = $(PWD)/libavif
LIBAVIF_BUILD = $(LIBAVIF_SRC)/build
LIBAVIF_DAV1D_SRC = $(LIBAVIF_SRC)/ext/dav1d
LIBAVIF_DAV1D_BUILD = $(LIBAVIF_DAV1D_SRC)/build
LIBAVIF_AOM_SRC = $(LIBAVIF_SRC)/ext/aom
LIBAVIF_AOM_BUILD = $(LIBAVIF_AOM_SRC)/build.libavif
LIBAVIF_YUV_SRC = $(LIBAVIF_SRC)/ext/libyuv
LIBAVIF_YUV_BUILD = $(LIBAVIF_YUV_SRC)/build

WASI_SDK_PATH = /opt/wasi-sdk
export CC = $(WASI_SDK_PATH)/bin/clang --sysroot=$(WASI_SDK_PATH)/share/wasi-sysroot --target=wasm32-wasi
export CFLAGS = -O3

CMAKE_TOOLCHAIN_FILE=$(WASI_SDK_PATH)/share/cmake/wasi-sdk.cmake

BIN := avif.wasm

all: $(BIN)

$(LIBAVIF_SRC):
	git clone -b $(LIBAVIF_VERSION) --depth 1 https://github.com/AOMediaCodec/libavif
	mkdir -p $(LIBAVIF_BUILD)
	test -d $@

$(LIBAVIF_DAV1D_SRC): $(LIBAVIF_SRC)
	cd $(LIBAVIF_SRC)/ext; \
	git clone -b $(LIBAVIF_DAV1D_VERSION) --depth 1 --recursive --jobs `nproc` https://github.com/MikuAuahDark/dav1d-cmake dav1d
	mkdir -p $(LIBAVIF_DAV1D_BUILD)
	test -d $@

$(LIBAVIF_AOM_SRC): $(LIBAVIF_SRC)
	cd $(LIBAVIF_SRC)/ext; \
	git clone -b $(LIBAVIF_AOM_VERSION) --depth 1 --recursive --jobs `nproc` https://aomedia.googlesource.com/aom
	mkdir -p $(LIBAVIF_AOM_BUILD)
	test -d $@

$(LIBAVIF_YUV_SRC): $(LIBAVIF_SRC)
	cd $(LIBAVIF_SRC)/ext; \
	git clone -b $(LIBAVIF_YUV_VERSION) --depth 1 https://chromium.googlesource.com/libyuv/libyuv
	sed -i '/^ADD_EXECUTABLE\|^TARGET_LINK_LIBRARIES/d' $(LIBAVIF_YUV_SRC)/CMakeLists.txt
	mkdir -p $(LIBAVIF_YUV_BUILD)
	test -d $@

$(LIBAVIF_DAV1D_BUILD)/libdav1d.a: $(LIBAVIF_DAV1D_SRC)
	cd $(LIBAVIF_DAV1D_BUILD); \
	cmake $(LIBAVIF_DAV1D_SRC) \
		-DCMAKE_BUILD_TYPE=Release \
		-DBUILD_SHARED_LIBS=0 \
		-Denable_tools=0 \
		-Dbitdepth_16=1 \
		-DCMAKE_TOOLCHAIN_FILE=$(CMAKE_TOOLCHAIN_FILE)

	cd $(LIBAVIF_DAV1D_BUILD); \
	$(MAKE) -j$(shell nproc)

	mkdir -p $(LIBAVIF_DAV1D_BUILD)/src && cp $(LIBAVIF_DAV1D_BUILD)/libdav1d.a $(LIBAVIF_DAV1D_BUILD)/src/
	ln -sf $(LIBAVIF_DAV1D_SRC)/dav1d/include $(LIBAVIF_DAV1D_SRC)/include

$(LIBAVIF_AOM_BUILD)/libaom.a: $(LIBAVIF_AOM_SRC)
	cd $(LIBAVIF_AOM_BUILD); \
	cmake $(LIBAVIF_AOM_SRC) \
		-DCMAKE_BUILD_TYPE=Release \
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
		-DCONFIG_MULTITHREAD=0 \
		-DCMAKE_TOOLCHAIN_FILE=$(CMAKE_TOOLCHAIN_FILE)

	cd $(LIBAVIF_AOM_BUILD); \
	$(MAKE) -j$(shell nproc)

$(LIBAVIF_YUV_BUILD)/libyuv.a: $(LIBAVIF_YUV_SRC)
	cd $(LIBAVIF_YUV_BUILD); \
	cmake $(LIBAVIF_YUV_SRC) \
		-DCMAKE_BUILD_TYPE=Release \
		-DBUILD_SHARED_LIBS=0 \
		-DCMAKE_TOOLCHAIN_FILE=$(CMAKE_TOOLCHAIN_FILE)

	cd $(LIBAVIF_YUV_BUILD); \
	$(MAKE) -j$(shell nproc)

$(LIBAVIF_BUILD)/libavif.a: $(LIBAVIF_DAV1D_BUILD)/libdav1d.a $(LIBAVIF_AOM_BUILD)/libaom.a $(LIBAVIF_YUV_BUILD)/libyuv.a
	cd $(LIBAVIF_BUILD); \
	cmake $(LIBAVIF_SRC) \
		-DCMAKE_BUILD_TYPE=Release \
		-DBUILD_SHARED_LIBS=0 \
		-DAVIF_CODEC_DAV1D=1 \
		-DAVIF_LOCAL_DAV1D=1 \
		-DAVIF_CODEC_AOM=1 \
		-DAVIF_LOCAL_AOM=1 \
		-DAVIF_CODEC_AOM_DECODE=0 \
		-DAVIF_CODEC_AOM_ENCODE=1 \
		-DAVIF_LOCAL_LIBYUV=1 \
		-DCMAKE_TOOLCHAIN_FILE=$(CMAKE_TOOLCHAIN_FILE)

	cd $(LIBAVIF_BUILD); \
	$(MAKE) -j$(shell nproc)

$(BIN): $(LIBAVIF_BUILD)/libavif.a
	$(CC) \
		-O3 \
		-Wl,--no-entry \
		-mexec-model=reactor \
		-mnontrapping-fptoint \
		-I $(LIBAVIF_SRC)/include \
		-o $@ \
		-Wall \
		avif.c \
		${LIBAVIF_BUILD}/libavif.a \
		${LIBAVIF_YUV_BUILD}/libyuv.a \
		${LIBAVIF_DAV1D_BUILD}/libdav1d.a \
		${LIBAVIF_DAV1D_BUILD}/libdav1d_bitdepth_8.a \
		${LIBAVIF_DAV1D_BUILD}/libdav1d_bitdepth_16.a \
		${LIBAVIF_AOM_BUILD}/libaom.a

.PHONY: clean

clean:
	-rm -rf $(LIBAVIF_SRC)
