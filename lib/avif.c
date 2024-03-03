#include <stdlib.h>
#include <string.h>

#include "avif/avif.h"

void *allocate(size_t size);
void deallocate(void *ptr);

int decode(uint8_t *avif_in, int avif_in_size, int config_only, uint32_t *width, uint32_t *height, uint32_t *depth, uint8_t *rgb_out);

__attribute__((export_name("allocate")))
void *allocate(size_t size) {
    return malloc(size);
}

__attribute__((export_name("deallocate")))
void deallocate(void *ptr) {
    free(ptr);
}

__attribute__((export_name("decode")))
int decode(uint8_t *avif_in, int avif_in_size, int config_only, uint32_t *width, uint32_t *height, uint32_t *depth, uint8_t *rgb_out) {
    avifRGBImage rgb;

    avifDecoder *decoder = avifDecoderCreate();
    decoder->ignoreExif = 1;
    decoder->ignoreXMP = 1;

    avifResult result = avifDecoderSetIOMemory(decoder, avif_in, avif_in_size);
    if(result != AVIF_RESULT_OK) {
        avifDecoderDestroy(decoder);
        return 0;
    }

    result = avifDecoderParse(decoder);
    if(result != AVIF_RESULT_OK) {
        avifDecoderDestroy(decoder);
        return 0;
    }

    *width = (uint32_t)decoder->image->width;
    *height = (uint32_t)decoder->image->height;
    *depth = (uint32_t)decoder->image->depth;

    if(config_only) {
        avifDecoderDestroy(decoder);
        return 1;
    }

    if(avifDecoderNextImage(decoder) == AVIF_RESULT_OK) {
        avifRGBImageSetDefaults(&rgb, decoder->image);

        result = avifRGBImageAllocatePixels(&rgb);
        if(result != AVIF_RESULT_OK) {
            avifDecoderDestroy(decoder);
            return 0;
        }

        result = avifImageYUVToRGB(decoder->image, &rgb);
        if(result != AVIF_RESULT_OK) {
            avifRGBImageFreePixels(&rgb);
            avifDecoderDestroy(decoder);
            return 0;
        }

        int buf_size = rgb.rowBytes * rgb.height;
        memcpy(rgb_out, rgb.pixels, buf_size);

        avifRGBImageFreePixels(&rgb);
        avifDecoderDestroy(decoder);
        return 1;
    }

    avifDecoderDestroy(decoder);
    return 0;
}
