// Package render turns a solved layout tree into pixels.
package render

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Photo is an input image as the layout sees it: upright, after EXIF rotation.
type Photo struct {
	Path   string
	src    string // file actually decoded (a converted copy for HEIC/WebP on macOS)
	W, H   int    // upright size
	Orient int    // EXIF orientation 1..8
}

func (p Photo) Aspect() float64 { return float64(p.W) / float64(p.H) }

// Probe reads only the header and EXIF of an image, not its pixels.
func Probe(path string) (Photo, error) {
	src, err := decodable(path)
	if err != nil {
		return Photo{}, err
	}
	f, err := os.Open(src)
	if err != nil {
		return Photo{}, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return Photo{}, fmt.Errorf("%s: not a JPEG, PNG or GIF image (%v)", path, err)
	}
	head := make([]byte, 128<<10)
	n, _ := f.ReadAt(head, 0)
	o := exifOrientation(head[:n])
	p := Photo{Path: path, src: src, W: cfg.Width, H: cfg.Height, Orient: o}
	if o >= 5 {
		p.W, p.H = p.H, p.W
	}
	return p, nil
}

// decodable returns a path the standard library can decode. On macOS, HEIC
// and WebP files (the iPhone default) are converted with the built-in sips tool.
func decodable(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".heic", ".heif", ".webp", ".avif", ".tif", ".tiff":
	default:
		return path, nil
	}
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("%s: %s files need converting to JPEG or PNG first", path, ext)
	}
	dir, err := os.MkdirTemp("", "collage-")
	if err != nil {
		return "", err
	}
	out := filepath.Join(dir, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))+".jpg")
	if msg, err := exec.Command("sips", "-s", "format", "jpeg", path, "--out", out).CombinedOutput(); err != nil {
		return "", fmt.Errorf("%s: could not convert with sips: %s", path, bytes.TrimSpace(msg))
	}
	return out, nil
}

// Load decodes the full image (still in sensor orientation).
func (p Photo) Load() (image.Image, error) {
	f, err := os.Open(p.src)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// Cleanup removes temporary converted copies.
func (p Photo) Cleanup() {
	if p.src != p.Path {
		os.RemoveAll(filepath.Dir(p.src))
	}
}

// exifOrientation finds tag 0x0112 in a JPEG's APP1 Exif block; 1 if absent.
func exifOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(b) {
		if b[i] != 0xFF {
			return 1
		}
		marker := b[i+1]
		if marker == 0xDA || marker == 0xD9 { // start of scan / end of image
			return 1
		}
		size := int(binary.BigEndian.Uint16(b[i+2:]))
		seg := b[i+4 : min(len(b), i+2+size)]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			if o, err := tiffOrientation(seg[6:]); err == nil {
				return o
			}
			return 1
		}
		i += 2 + size
	}
	return 1
}

func tiffOrientation(t []byte) (int, error) {
	if len(t) < 8 {
		return 0, errors.New("short")
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 0, errors.New("bad byte order")
	}
	ifd := int(bo.Uint32(t[4:]))
	if ifd+2 > len(t) {
		return 0, errors.New("bad ifd")
	}
	n := int(bo.Uint16(t[ifd:]))
	for k := 0; k < n; k++ {
		e := ifd + 2 + 12*k
		if e+12 > len(t) {
			break
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			o := int(bo.Uint16(t[e+8:]))
			if o >= 1 && o <= 8 {
				return o, nil
			}
		}
	}
	return 1, nil
}
