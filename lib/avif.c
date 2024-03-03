#include <stdlib.h>
#include <string.h>

#include "avif/avif.h"

void *allocate(size_t size);
void deallocate(void *ptr);

int decode(uint8_t *avif_in, int avif_in_size, int config_only, int decode_all, uint32_t *width, uint32_t *height,
    uint32_t *depth, uint32_t *count, uint8_t *delay, uint8_t *rgb_out);

__attribute__((export_name("allocate")))
void *allocate(size_t size) {
    return malloc(size);
}

__attribute__((export_name("deallocate")))
void deallocate(void *ptr) {
    free(ptr);
}

__attribute__((export_name("decode")))
int decode(uint8_t *avif_in, int avif_in_size, int config_only, int decode_all, uint32_t *width, uint32_t *height,
    uint32_t *depth, uint32_t *count, uint8_t *delay, uint8_t *rgb_out) {
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
    *count = (uint32_t)decoder->imageCount;

    if(config_only) {
        avifDecoderDestroy(decoder);
        return 1;
    }

    for(int i = 0; i < decoder->imageCount; i++) {
        result = avifDecoderNextImage(decoder);
        if(result != AVIF_RESULT_OK) {
            avifDecoderDestroy(decoder);
            return 0;
        }

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
        memcpy(rgb_out + buf_size*decoder->imageIndex, rgb.pixels, buf_size);

        memcpy(delay + sizeof(double)*i, &decoder->imageTiming.duration, sizeof(double));

        avifRGBImageFreePixels(&rgb);

        if(!decode_all) {
            avifDecoderDestroy(decoder);
            return 1;
        }
    }

    avifDecoderDestroy(decoder);
    return 1;
}
