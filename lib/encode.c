#include <stdlib.h>

#include "avif/avif.h"

uint8_t* encode(uint8_t *rgb_in, int width, int height, size_t *size, int quality, int quality_alpha, int speed, int chroma);

uint8_t* encode(uint8_t *rgb_in, int width, int height, size_t *size, int quality, int quality_alpha, int speed, int chroma) {
    avifResult result;

    avifImage *image = avifImageCreate(width, height, 8, chroma);

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

int setjmp(int a) {
    return 0;
}

void longjmp(int a, int b) {
}
