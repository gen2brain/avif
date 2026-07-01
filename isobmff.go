package avif

import "encoding/binary"

// avifProps holds the primary item's stored size, bit depth and EXIF orientation.
type avifProps struct {
	width       int
	height      int
	hiDepth     bool
	orientation int
}

type ipcoProp struct {
	typ  string
	data []byte
}

// parseAVIFProps returns the primary item's dimensions, depth and orientation from an ISOBMFF prefix.
func parseAVIFProps(data []byte) (avifProps, bool) {
	p := avifProps{orientation: 1}

	meta, ok := metaPayload(data)
	if !ok {
		return p, false
	}

	ipco, ipma := iprpBoxes(meta)
	if ipco == nil {
		return p, false
	}

	props := ipcoProps(ipco)

	indices := ipmaIndices(ipma, primaryItem(meta))
	if len(indices) == 0 {
		for i := range props {
			indices = append(indices, i+1)
		}
	}

	var haveDim, haveRot, haveMir bool
	var angle, axis int

	for _, idx := range indices {
		if idx < 1 || idx > len(props) {
			continue
		}

		pr := props[idx-1]
		switch pr.typ {
		case "ispe":
			if len(pr.data) >= 12 {
				p.width = int(binary.BigEndian.Uint32(pr.data[4:8]))
				p.height = int(binary.BigEndian.Uint32(pr.data[8:12]))
				haveDim = true
			}
		case "pixi":
			if len(pr.data) >= 5 {
				n := int(pr.data[4])
				for k := 0; k < n && 5+k < len(pr.data); k++ {
					if pr.data[5+k] > 8 {
						p.hiDepth = true
					}
				}
			}
		case "irot":
			if len(pr.data) >= 1 {
				angle = int(pr.data[0] & 0x3)
				haveRot = true
			}
		case "imir":
			if len(pr.data) >= 1 {
				axis = int(pr.data[0] & 0x1)
				haveMir = true
			}
		}
	}

	p.orientation = exifOrientationFromIrotImir(haveRot, angle, haveMir, axis)

	return p, haveDim
}

// eachBox iterates the child boxes within b, invoking fn(type, payload) until fn returns false.
func eachBox(b []byte, fn func(typ string, payload []byte) bool) {
	off := 0
	for off+8 <= len(b) {
		size := int(binary.BigEndian.Uint32(b[off : off+4]))
		typ := string(b[off+4 : off+8])
		hdr := 8

		if size == 1 {
			if off+16 > len(b) {
				return
			}
			size = int(binary.BigEndian.Uint64(b[off+8 : off+16]))
			hdr = 16
		} else if size == 0 {
			size = len(b) - off
		}

		if size < hdr || off+size > len(b) {
			return
		}

		if !fn(typ, b[off+hdr:off+size]) {
			return
		}

		off += size
	}
}

// metaPayload returns the child-box region of the top-level meta box, past its version/flags.
func metaPayload(data []byte) ([]byte, bool) {
	var out []byte
	var ok bool

	eachBox(data, func(typ string, payload []byte) bool {
		if typ == "meta" {
			if len(payload) >= 4 {
				out = payload[4:]
				ok = true
			}
			return false
		}
		return true
	})

	return out, ok
}

// primaryItem returns the primary item ID from the pitm box, or -1 when absent.
func primaryItem(meta []byte) int {
	id := -1

	eachBox(meta, func(typ string, payload []byte) bool {
		if typ != "pitm" {
			return true
		}

		if payload[0] == 0 && len(payload) >= 6 {
			id = int(binary.BigEndian.Uint16(payload[4:6]))
		} else if len(payload) >= 8 {
			id = int(binary.BigEndian.Uint32(payload[4:8]))
		}

		return false
	})

	return id
}

// iprpBoxes returns the ipco and ipma payloads from the iprp box.
func iprpBoxes(meta []byte) (ipco, ipma []byte) {
	eachBox(meta, func(typ string, payload []byte) bool {
		if typ != "iprp" {
			return true
		}

		eachBox(payload, func(t string, p []byte) bool {
			switch t {
			case "ipco":
				ipco = p
			case "ipma":
				ipma = p
			}
			return true
		})

		return false
	})

	return
}

// ipcoProps returns the ordered property boxes of the ipco container (1-based index).
func ipcoProps(ipco []byte) []ipcoProp {
	var out []ipcoProp

	eachBox(ipco, func(typ string, payload []byte) bool {
		out = append(out, ipcoProp{typ, payload})
		return true
	})

	return out
}

// ipmaIndices returns the property indices associated with item from the ipma box.
func ipmaIndices(ipma []byte, item int) []int {
	if len(ipma) < 8 || item < 0 {
		return nil
	}

	version := ipma[0]
	wide := ipma[3]&1 != 0

	off := 4
	count := int(binary.BigEndian.Uint32(ipma[off : off+4]))
	off += 4

	for i := 0; i < count; i++ {
		var id int
		if version < 1 {
			if off+2 > len(ipma) {
				return nil
			}
			id = int(binary.BigEndian.Uint16(ipma[off : off+2]))
			off += 2
		} else {
			if off+4 > len(ipma) {
				return nil
			}
			id = int(binary.BigEndian.Uint32(ipma[off : off+4]))
			off += 4
		}

		if off >= len(ipma) {
			return nil
		}

		assoc := int(ipma[off])
		off++

		var indices []int
		for j := 0; j < assoc; j++ {
			var idx int
			if wide {
				if off+2 > len(ipma) {
					return nil
				}
				idx = int(binary.BigEndian.Uint16(ipma[off:off+2]) & 0x7fff)
				off += 2
			} else {
				if off >= len(ipma) {
					return nil
				}
				idx = int(ipma[off] & 0x7f)
				off++
			}
			indices = append(indices, idx)
		}

		if id == item {
			return indices
		}
	}

	return nil
}
