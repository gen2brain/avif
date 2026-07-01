package avif

import "image"

// exifOrientationFromIrotImir maps irot/imir to the equivalent EXIF orientation (per libavif avifexif.c).
func exifOrientationFromIrotImir(haveRot bool, angle int, haveMir bool, axis int) int {
	if haveRot && angle == 1 {
		if haveMir {
			if axis != 0 {
				return 7
			}
			return 5
		}
		return 8
	}

	if haveRot && angle == 2 {
		if haveMir {
			if axis != 0 {
				return 4
			}
			return 2
		}
		return 3
	}

	if haveRot && angle == 3 {
		if haveMir {
			if axis != 0 {
				return 5
			}
			return 7
		}
		return 6
	}

	if haveMir {
		if axis != 0 {
			return 2
		}
		return 4
	}

	return 1
}

// applyOrientation returns img rotated/flipped per the EXIF orientation; unchanged for 1 or an unhandled type.
func applyOrientation(img image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return img
	}

	switch src := img.(type) {
	case *image.RGBA:
		return orientPix(src.Pix, src.Stride, src.Rect, orientation, 4)
	case *image.RGBA64:
		return orientPix(src.Pix, src.Stride, src.Rect, orientation, 8)
	default:
		return img
	}
}

// orientPix builds a reoriented image by copying pixels of the given byte width.
func orientPix(pix []byte, stride int, r image.Rectangle, o, bpp int) image.Image {
	sw, sh := r.Dx(), r.Dy()
	dw, dh := sw, sh
	if o >= 5 {
		dw, dh = sh, sw
	}

	dstStride := dw * bpp
	dst := make([]byte, dstStride*dh)

	for sy := 0; sy < sh; sy++ {
		srow := pix[sy*stride:]
		for sx := 0; sx < sw; sx++ {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = sw-1-sx, sy
			case 3:
				dx, dy = sw-1-sx, sh-1-sy
			case 4:
				dx, dy = sx, sh-1-sy
			case 5:
				dx, dy = sy, sx
			case 6:
				dx, dy = sh-1-sy, sx
			case 7:
				dx, dy = sh-1-sy, sw-1-sx
			case 8:
				dx, dy = sy, sw-1-sx
			}

			si := sx * bpp
			di := dy*dstStride + dx*bpp
			copy(dst[di:di+bpp], srow[si:si+bpp])
		}
	}

	rect := image.Rect(0, 0, dw, dh)
	if bpp == 8 {
		return &image.RGBA64{Pix: dst, Stride: dstStride, Rect: rect}
	}

	return &image.RGBA{Pix: dst, Stride: dstStride, Rect: rect}
}
