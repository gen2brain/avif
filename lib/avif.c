#include <stdlib.h>
#include <string.h>

#include "avif/avif.h"

void* allocate(size_t size);
void deallocate(void *ptr);

int decode(uint8_t *avif_in, int avif_in_size, int config_only, int decode_all, uint32_t *width, uint32_t *height, uint32_t *depth, uint32_t *count, uint8_t *delay, uint8_t *rgb_out);
uint8_t* encode(uint8_t *rgb_in, int width, int height, size_t *size, int quality, int quality_alpha, int speed);

__attribute__((export_name("allocate")))
void* allocate(size_t size) {
    return malloc(size);
}

__attribute__((export_name("deallocate")))
void deallocate(void *ptr) {
    free(ptr);
}

__attribute__((export_name("decode")))
int decode(uint8_t *avif_in, int avif_in_size, int config_only, int decode_all, uint32_t *width, uint32_t *height,
    uint32_t *depth, uint32_t *count, uint8_t *delay, uint8_t *rgb_out) {

    avifDecoder *decoder = avifDecoderCreate();
    decoder->ignoreExif = 1;
    decoder->ignoreXMP = 1;
    decoder->maxThreads = 0;
    decoder->strictFlags = 0;

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

    avifRGBImage rgb;
    avifRGBImageSetDefaults(&rgb, decoder->image);

    rgb.maxThreads = 0;
    rgb.alphaPremultiplied = 1;

    if(decoder->image->depth > 8) {
        rgb.depth = 16;
    }

    if(decoder->imageCount > 1 && decode_all) {
        rgb.chromaUpsampling = AVIF_CHROMA_UPSAMPLING_FASTEST;
    }

    while(avifDecoderNextImage(decoder) == AVIF_RESULT_OK) {
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

        memcpy(delay + sizeof(double)*decoder->imageIndex, &decoder->imageTiming.duration, sizeof(double));

        avifRGBImageFreePixels(&rgb);

        if(!decode_all) {
            avifDecoderDestroy(decoder);
            return 1;
        }
    }

    avifDecoderDestroy(decoder);
    return 1;
}

__attribute__((export_name("encode")))
uint8_t* encode(uint8_t *rgb_in, int width, int height, size_t *size, int quality, int quality_alpha, int speed) {
    avifResult result;

    avifImage *image = avifImageCreate(width, height, 8, AVIF_PIXEL_FORMAT_YUV420);

    avifRGBImage rgb;
    avifRGBImageSetDefaults(&rgb, image);

    rgb.maxThreads = 0;
    rgb.alphaPremultiplied = 1;

    result = avifRGBImageAllocatePixels(&rgb);
    if(result != AVIF_RESULT_OK) {
        avifImageDestroy(image);
        return 0;
    }

    rgb.pixels = rgb_in;

    result = avifImageRGBToYUV(image, &rgb);
    if(result != AVIF_RESULT_OK) {
        avifImageDestroy(image);
        avifRGBImageFreePixels(&rgb);
        return 0;
    }

    avifRWData output = AVIF_DATA_EMPTY;

    avifEncoder *encoder = avifEncoderCreate();
    encoder->maxThreads = 0;
    encoder->quality = quality;
    encoder->qualityAlpha = quality_alpha;
    encoder->speed = speed;

    result = avifEncoderAddImage(encoder, image, 1, AVIF_ADD_IMAGE_FLAG_SINGLE);
    if(result != AVIF_RESULT_OK) {
        avifImageDestroy(image);
        avifRGBImageFreePixels(&rgb);
        avifEncoderDestroy(encoder);
        return 0;
    }

    result = avifEncoderFinish(encoder, &output);
    if(result != AVIF_RESULT_OK) {
        avifImageDestroy(image);
        avifRGBImageFreePixels(&rgb);
        avifEncoderDestroy(encoder);
        return 0;
    }

    *size = output.size;

    avifImageDestroy(image);
    avifRGBImageFreePixels(&rgb);
    avifEncoderDestroy(encoder);

    return output.data;
}


int pthread_create(int a, int b, int c, int d) {
    return 0;
}

int pthread_join(int a, int b) {
    return 0;
}

int pthread_once(int a, int b) {
    return 0;
}

int pthread_mutex_init(int a, int b) {
    return 0;
}

int pthread_mutex_lock(int a) {
    return 0;
}

int pthread_mutex_unlock(int a) {
    return 0;
}

int pthread_mutex_destroy(int a) {
    return 0;
}

int pthread_cond_init(int a, int b) {
    return 0;
}

int pthread_cond_signal(int a) {
    return 0;
}

int pthread_cond_wait(int a, int b) {
    return 0;
}

int pthread_cond_broadcast(int a) {
    return 0;
}

int pthread_cond_destroy(int a) {
    return 0;
}

int pthread_attr_init(int a) {
    return 0;
}

int pthread_attr_setstacksize(int a, int b) {
    return 0;
}

int pthread_attr_destroy(int a) {
    return 0;
}

int setjmp(int a) {
    return 0;
}

void longjmp(int a, int b) {
}
