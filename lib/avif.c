#include <stdlib.h>
#include <string.h>

#include "avif/avif.h"
#include <emscripten.h>

void *allocate(size_t size);
void deallocate(void *ptr);

void rgba(avifImage *image, uint8_t *rgb_out);
avifImage *decode(uint8_t *avif_in, int avif_in_size, int config_only, uint32_t *width, uint32_t *height);

EMSCRIPTEN_KEEPALIVE
void *allocate(size_t size) {
    return malloc(size);
}

EMSCRIPTEN_KEEPALIVE
void deallocate(void *ptr) {
    free(ptr);
}

EMSCRIPTEN_KEEPALIVE
void rgba(avifImage *image, uint8_t *rgb_out) {
    avifRGBImage rgb;
    avifRGBImageSetDefaults(&rgb, image);  // Defaults to AVIF_RGB_FORMAT_RGBA.
    rgb.depth = 8;

    avifRGBImageAllocatePixels(&rgb);
    avifImageYUVToRGB(image, &rgb);

    int buf_size = image->width * image->height * 4;
    memcpy(rgb_out, rgb.pixels, buf_size);

    avifRGBImageFreePixels(&rgb);
    avifImageDestroy(image);
}

EMSCRIPTEN_KEEPALIVE
avifImage* decode(uint8_t *avif_in, int avif_in_size, int config_only, uint32_t *width, uint32_t *height) {
    avifImage *image = avifImageCreateEmpty();
    avifDecoder *decoder = avifDecoderCreate();

    avifResult result = avifDecoderReadMemory(decoder, image, avif_in, avif_in_size);
    avifDecoderDestroy(decoder);

    if(result == AVIF_RESULT_OK) {
        *width = (uint32_t)image->width;
        *height = (uint32_t)image->height;
    }

    if(config_only) {
        avifImageDestroy(image);
    }

    return image;
}
