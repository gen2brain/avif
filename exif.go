package avif

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// ErrNoExif is returned by DecodeExif when the AVIF has no Exif item.
var ErrNoExif = errors.New("avif: no exif data")

// Exif holds the EXIF metadata decoded from an AVIF image.
type Exif struct {
	// Basic image info
	Orientation int // EXIF orientation (1-8). 1 = normal, values 2-8 indicate rotation/flip.
	Width       int // Image width in pixels.
	Height      int // Image height in pixels.

	// Camera info
	Make     string // Camera manufacturer (e.g., "Canon").
	Model    string // Camera model (e.g., "Canon EOS 5D Mark III").
	Software string // Software used to process/create the image.

	// Date/Time (format: "YYYY:MM:DD HH:MM:SS")
	DateTime         string // File modification date/time.
	DateTimeOriginal string // Original capture date/time (when photo was taken).

	// Exposure settings
	ExposureTime float64 // Shutter speed in seconds (e.g., 0.004 = 1/250s).
	FNumber      float64 // Aperture f-number (e.g., 5.6 = f/5.6).
	ISOSpeed     int     // ISO speed rating (e.g., 800).
	FocalLength  float64 // Lens focal length in millimeters.
	Flash        int     // Flash mode/status (0 = no flash, non-zero = flash fired).

	// GPS location
	GPSLatitude  float64 // Latitude in decimal degrees (positive = North, negative = South).
	GPSLongitude float64 // Longitude in decimal degrees (positive = East, negative = West).
	GPSAltitude  float64 // Altitude in meters above sea level.

	// Copyright/Author
	Copyright string // Copyright notice.
	Artist    string // Creator/photographer name.
}

// DecodeExif reads the EXIF metadata from an AVIF image. It returns ErrNoExif if the image carries no Exif item.
func DecodeExif(r io.Reader) (*Exif, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("avif: read: %w", err)
	}

	tiff := exifPayload(data)
	if tiff == nil {
		return nil, ErrNoExif
	}

	exif := &Exif{Orientation: 1}
	if err := parseExifData(tiff, exif); err != nil {
		return nil, fmt.Errorf("avif: %w", err)
	}

	return exif, nil
}

// exifPayload returns the TIFF payload of the AVIF Exif item, or nil if absent.
func exifPayload(data []byte) []byte {
	meta, ok := metaPayload(data)
	if !ok {
		return nil
	}

	id := exifItemID(meta)
	if id < 0 {
		return nil
	}

	off, length, method, ok := ilocExtent(meta, id)
	if !ok || length < 4 {
		return nil
	}

	var raw []byte
	switch method {
	case 0:
		if off+length > uint64(len(data)) {
			return nil
		}
		raw = data[off : off+length]
	case 1:
		idat := idatPayload(meta)
		if idat == nil || off+length > uint64(len(idat)) {
			return nil
		}
		raw = idat[off : off+length]
	default:
		return nil
	}

	start := 4 + int(binary.BigEndian.Uint32(raw[0:4]))
	if start >= len(raw) {
		return nil
	}

	return raw[start:]
}

// exifItemID returns the item ID of the Exif item from the iinf box, or -1 when absent.
func exifItemID(meta []byte) int {
	id := -1

	eachBox(meta, func(typ string, payload []byte) bool {
		if typ != "iinf" {
			return true
		}

		start := 6
		if payload[0] != 0 {
			start = 8
		}
		if start > len(payload) {
			return false
		}

		eachBox(payload[start:], func(t string, p []byte) bool {
			if t != "infe" || len(p) < 1 {
				return true
			}

			var itemID int
			var itemType string
			if p[0] == 2 && len(p) >= 12 {
				itemID = int(binary.BigEndian.Uint16(p[4:6]))
				itemType = string(p[8:12])
			} else if p[0] >= 3 && len(p) >= 14 {
				itemID = int(binary.BigEndian.Uint32(p[4:8]))
				itemType = string(p[10:14])
			} else {
				return true
			}

			if itemType == "Exif" {
				id = itemID
				return false
			}

			return true
		})

		return false
	})

	return id
}

// ilocExtent returns the first extent's absolute offset, length and construction method for item.
func ilocExtent(meta []byte, item int) (offset, length uint64, method int, ok bool) {
	eachBox(meta, func(typ string, p []byte) bool {
		if typ != "iloc" || len(p) < 8 {
			return true
		}

		version := p[0]
		offsetSize := int(p[4] >> 4)
		lengthSize := int(p[4] & 0xf)
		baseOffsetSize := int(p[5] >> 4)
		indexSize := int(p[5] & 0xf)

		off := 8
		var itemCount int
		if version < 2 {
			itemCount = int(binary.BigEndian.Uint16(p[6:8]))
		} else {
			if len(p) < 10 {
				return false
			}
			itemCount = int(binary.BigEndian.Uint32(p[6:10]))
			off = 10
		}

		for i := 0; i < itemCount; i++ {
			var id int
			if version < 2 {
				if off+2 > len(p) {
					return false
				}
				id = int(binary.BigEndian.Uint16(p[off : off+2]))
				off += 2
			} else {
				if off+4 > len(p) {
					return false
				}
				id = int(binary.BigEndian.Uint32(p[off : off+4]))
				off += 4
			}

			m := 0
			if version == 1 || version == 2 {
				if off+2 > len(p) {
					return false
				}
				m = int(binary.BigEndian.Uint16(p[off:off+2]) & 0xf)
				off += 2
			}

			off += 2 // data_reference_index

			base, good := readUint(p, &off, baseOffsetSize)
			if !good {
				return false
			}

			if off+2 > len(p) {
				return false
			}
			extents := int(binary.BigEndian.Uint16(p[off : off+2]))
			off += 2

			var eo, el uint64
			for e := 0; e < extents; e++ {
				if (version == 1 || version == 2) && indexSize > 0 {
					if _, good = readUint(p, &off, indexSize); !good {
						return false
					}
				}

				o, ok1 := readUint(p, &off, offsetSize)
				l, ok2 := readUint(p, &off, lengthSize)
				if !ok1 || !ok2 {
					return false
				}
				if e == 0 {
					eo, el = o, l
				}
			}

			if id == item {
				offset, length, method, ok = base+eo, el, m, true
				return false
			}
		}

		return false
	})

	return offset, length, method, ok
}

func idatPayload(meta []byte) []byte {
	var out []byte

	eachBox(meta, func(typ string, p []byte) bool {
		if typ == "idat" {
			out = p
			return false
		}
		return true
	})

	return out
}

func readUint(b []byte, off *int, size int) (uint64, bool) {
	if size == 0 {
		return 0, true
	}
	if *off+size > len(b) {
		return 0, false
	}

	var v uint64
	for i := 0; i < size; i++ {
		v = v<<8 | uint64(b[*off+i])
	}
	*off += size

	return v, true
}

// EXIF tag constants
const (
	tagOrientation    = 0x0112
	tagImageWidth     = 0x0100
	tagImageLength    = 0x0101
	tagMake           = 0x010F
	tagModel          = 0x0110
	tagSoftware       = 0x0131
	tagDateTime       = 0x0132
	tagArtist         = 0x013B
	tagCopyright      = 0x8298
	tagExifIFDPointer = 0x8769
	tagGPSIFDPointer  = 0x8825

	tagExposureTime     = 0x829A
	tagFNumber          = 0x829D
	tagISOSpeedRatings  = 0x8827
	tagDateTimeOriginal = 0x9003
	tagFlash            = 0x9209
	tagFocalLength      = 0x920A

	tagGPSLatitudeRef  = 0x0001
	tagGPSLatitude     = 0x0002
	tagGPSLongitudeRef = 0x0003
	tagGPSLongitude    = 0x0004
	tagGPSAltitudeRef  = 0x0005
	tagGPSAltitude     = 0x0006
)

// EXIF data type constants
const (
	typeUnsignedByte     = 1
	typeASCIIString      = 2
	typeUnsignedShort    = 3
	typeUnsignedLong     = 4
	typeUnsignedRational = 5
	typeSignedByte       = 6
	typeUndefined        = 7
	typeSignedShort      = 8
	typeSignedLong       = 9
	typeSignedRational   = 10
	typeSingleFloat      = 11
	typeDoubleFloat      = 12
)

// exifReader wraps the EXIF data with helper functions for reading different data types
type exifReader struct {
	data         []byte
	littleEndian bool
}

func (r *exifReader) uint16(offset int) uint16 {
	if offset+1 >= len(r.data) {
		return 0
	}
	if r.littleEndian {
		return uint16(r.data[offset]) | (uint16(r.data[offset+1]) << 8)
	}
	return (uint16(r.data[offset]) << 8) | uint16(r.data[offset+1])
}

func (r *exifReader) uint32(offset int) uint32 {
	if offset+3 >= len(r.data) {
		return 0
	}
	if r.littleEndian {
		return uint32(r.data[offset]) | (uint32(r.data[offset+1]) << 8) |
			(uint32(r.data[offset+2]) << 16) | (uint32(r.data[offset+3]) << 24)
	}
	return (uint32(r.data[offset]) << 24) | (uint32(r.data[offset+1]) << 16) |
		(uint32(r.data[offset+2]) << 8) | uint32(r.data[offset+3])
}

func (r *exifReader) readString(offset, maxLen int) string {
	if offset >= len(r.data) {
		return ""
	}
	end := offset
	for end < len(r.data) && end < offset+maxLen && r.data[end] != 0 {
		end++
	}
	return string(r.data[offset:end])
}

func (r *exifReader) readRational(offset int) float64 {
	if offset+7 >= len(r.data) {
		return 0
	}
	numerator := r.uint32(offset)
	denominator := r.uint32(offset + 4)
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

// parseExifData parses the TIFF/EXIF data structure and populates the Exif struct
func parseExifData(data []byte, exif *Exif) error {
	if len(data) < 8 {
		return fmt.Errorf("EXIF data too short")
	}

	reader := &exifReader{data: data}

	if data[0] == 0x49 && data[1] == 0x49 {
		reader.littleEndian = true
	} else if data[0] == 0x4D && data[1] == 0x4D {
		reader.littleEndian = false
	} else {
		return fmt.Errorf("invalid EXIF byte order marker")
	}

	if reader.uint16(2) != 42 {
		return fmt.Errorf("invalid EXIF magic number")
	}

	ifdOffset := reader.uint32(4)
	if ifdOffset < 8 || int(ifdOffset) >= len(data) {
		return fmt.Errorf("invalid IFD offset")
	}

	exifIFDOffset, gpsIFDOffset := parseIFD(reader, int(ifdOffset), exif)

	if exifIFDOffset > 0 {
		parseExifSubIFD(reader, exifIFDOffset, exif)
	}

	if gpsIFDOffset > 0 {
		parseGPSSubIFD(reader, gpsIFDOffset, exif)
	}

	return nil
}

// parseIFD parses an Image File Directory and returns pointers to EXIF and GPS SubIFDs
func parseIFD(reader *exifReader, offset int, exif *Exif) (exifIFDOffset, gpsIFDOffset int) {
	if offset+1 >= len(reader.data) {
		return 0, 0
	}

	numEntries := reader.uint16(offset)
	offset += 2

	for i := 0; i < int(numEntries); i++ {
		entryOffset := offset + i*12
		if entryOffset+11 >= len(reader.data) {
			break
		}

		tag := reader.uint16(entryOffset)
		dataType := reader.uint16(entryOffset + 2)
		count := reader.uint32(entryOffset + 4)
		valueOffset := entryOffset + 8

		dataSize := getDataSize(dataType, count)
		if dataSize > 4 {
			valueOffset = int(reader.uint32(valueOffset))
			if valueOffset >= len(reader.data) {
				continue
			}
		}

		switch tag {
		case tagOrientation:
			if dataType == typeUnsignedShort {
				exif.Orientation = int(reader.uint16(valueOffset))
			}
		case tagImageWidth:
			if dataType == typeUnsignedShort {
				exif.Width = int(reader.uint16(valueOffset))
			} else if dataType == typeUnsignedLong {
				exif.Width = int(reader.uint32(valueOffset))
			}
		case tagImageLength:
			if dataType == typeUnsignedShort {
				exif.Height = int(reader.uint16(valueOffset))
			} else if dataType == typeUnsignedLong {
				exif.Height = int(reader.uint32(valueOffset))
			}
		case tagMake:
			if dataType == typeASCIIString {
				exif.Make = reader.readString(valueOffset, int(count))
			}
		case tagModel:
			if dataType == typeASCIIString {
				exif.Model = reader.readString(valueOffset, int(count))
			}
		case tagSoftware:
			if dataType == typeASCIIString {
				exif.Software = reader.readString(valueOffset, int(count))
			}
		case tagDateTime:
			if dataType == typeASCIIString {
				exif.DateTime = reader.readString(valueOffset, int(count))
			}
		case tagArtist:
			if dataType == typeASCIIString {
				exif.Artist = reader.readString(valueOffset, int(count))
			}
		case tagCopyright:
			if dataType == typeASCIIString {
				exif.Copyright = reader.readString(valueOffset, int(count))
			}
		case tagExifIFDPointer:
			if dataType == typeUnsignedLong {
				exifIFDOffset = int(reader.uint32(valueOffset))
			}
		case tagGPSIFDPointer:
			if dataType == typeUnsignedLong {
				gpsIFDOffset = int(reader.uint32(valueOffset))
			}
		}
	}

	return exifIFDOffset, gpsIFDOffset
}

// parseExifSubIFD parses the EXIF SubIFD for camera settings
func parseExifSubIFD(reader *exifReader, offset int, exif *Exif) {
	if offset+1 >= len(reader.data) {
		return
	}

	numEntries := reader.uint16(offset)
	offset += 2

	for i := 0; i < int(numEntries); i++ {
		entryOffset := offset + i*12
		if entryOffset+11 >= len(reader.data) {
			break
		}

		tag := reader.uint16(entryOffset)
		dataType := reader.uint16(entryOffset + 2)
		count := reader.uint32(entryOffset + 4)
		valueOffset := entryOffset + 8

		dataSize := getDataSize(dataType, count)
		if dataSize > 4 {
			valueOffset = int(reader.uint32(valueOffset))
			if valueOffset >= len(reader.data) {
				continue
			}
		}

		switch tag {
		case tagExposureTime:
			if dataType == typeUnsignedRational {
				exif.ExposureTime = reader.readRational(valueOffset)
			}
		case tagFNumber:
			if dataType == typeUnsignedRational {
				exif.FNumber = reader.readRational(valueOffset)
			}
		case tagISOSpeedRatings:
			if dataType == typeUnsignedShort {
				exif.ISOSpeed = int(reader.uint16(valueOffset))
			}
		case tagDateTimeOriginal:
			if dataType == typeASCIIString {
				exif.DateTimeOriginal = reader.readString(valueOffset, int(count))
			}
		case tagFlash:
			if dataType == typeUnsignedShort {
				exif.Flash = int(reader.uint16(valueOffset))
			}
		case tagFocalLength:
			if dataType == typeUnsignedRational {
				exif.FocalLength = reader.readRational(valueOffset)
			}
		}
	}
}

// parseGPSSubIFD parses the GPS SubIFD for location data
func parseGPSSubIFD(reader *exifReader, offset int, exif *Exif) {
	if offset+1 >= len(reader.data) {
		return
	}

	numEntries := reader.uint16(offset)
	offset += 2

	var latRef, lonRef string
	var latValues, lonValues []float64

	for i := 0; i < int(numEntries); i++ {
		entryOffset := offset + i*12
		if entryOffset+11 >= len(reader.data) {
			break
		}

		tag := reader.uint16(entryOffset)
		dataType := reader.uint16(entryOffset + 2)
		count := reader.uint32(entryOffset + 4)
		valueOffset := entryOffset + 8

		dataSize := getDataSize(dataType, count)
		if dataSize > 4 {
			valueOffset = int(reader.uint32(valueOffset))
			if valueOffset >= len(reader.data) {
				continue
			}
		}

		switch tag {
		case tagGPSLatitudeRef:
			if dataType == typeASCIIString {
				latRef = reader.readString(valueOffset, 2)
			}
		case tagGPSLatitude:
			if dataType == typeUnsignedRational && count == 3 {
				latValues = []float64{
					reader.readRational(valueOffset),
					reader.readRational(valueOffset + 8),
					reader.readRational(valueOffset + 16),
				}
			}
		case tagGPSLongitudeRef:
			if dataType == typeASCIIString {
				lonRef = reader.readString(valueOffset, 2)
			}
		case tagGPSLongitude:
			if dataType == typeUnsignedRational && count == 3 {
				lonValues = []float64{
					reader.readRational(valueOffset),
					reader.readRational(valueOffset + 8),
					reader.readRational(valueOffset + 16),
				}
			}
		case tagGPSAltitude:
			if dataType == typeUnsignedRational {
				alt := reader.readRational(valueOffset)
				if entryOffset+12 < len(reader.data) {
					altRef := reader.data[valueOffset-4]
					if altRef == 1 {
						alt = -alt
					}
				}
				exif.GPSAltitude = alt
			}
		}
	}

	if len(latValues) == 3 {
		exif.GPSLatitude = latValues[0] + latValues[1]/60.0 + latValues[2]/3600.0
		if latRef == "S" {
			exif.GPSLatitude = -exif.GPSLatitude
		}
	}
	if len(lonValues) == 3 {
		exif.GPSLongitude = lonValues[0] + lonValues[1]/60.0 + lonValues[2]/3600.0
		if lonRef == "W" {
			exif.GPSLongitude = -exif.GPSLongitude
		}
	}
}

// getDataSize calculates the size in bytes for a given EXIF data type and count
func getDataSize(dataType uint16, count uint32) int {
	var componentSize int
	switch dataType {
	case typeUnsignedByte, typeSignedByte, typeASCIIString, typeUndefined:
		componentSize = 1
	case typeUnsignedShort, typeSignedShort:
		componentSize = 2
	case typeUnsignedLong, typeSignedLong, typeSingleFloat:
		componentSize = 4
	case typeUnsignedRational, typeSignedRational, typeDoubleFloat:
		componentSize = 8
	default:
		componentSize = 1
	}
	return componentSize * int(count)
}
