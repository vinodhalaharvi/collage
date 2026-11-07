package render

import (
	"image"
	"math"
	"testing"

	"github.com/vinodhalaharvi/collage/engine"
)

// The continuous map used for cropping (toSource) and the discrete map used
// for rotating pixels (orient) must describe the same EXIF transform.
func TestOrientationMapsAgree(t *testing.T) {
	const sw, sh = 5, 3 // stored size
	m := image.NewRGBA(image.Rect(0, 0, sw, sh))
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			m.Pix[m.PixOffset(x, y)] = uint8(y*sw + x)
		}
	}
	for o := 1; o <= 8; o++ {
		up := orient(m, o)
		p := Photo{W: up.Bounds().Dx(), H: up.Bounds().Dy(), Orient: o}
		for y := 0; y < p.H; y++ {
			for x := 0; x < p.W; x++ {
				u, v := toSource(p, float64(x)+0.5, float64(y)+0.5)
				want := m.Pix[m.PixOffset(int(math.Floor(u)), int(math.Floor(v)))]
				if got := up.Pix[up.PixOffset(x, y)]; got != want {
					t.Fatalf("orientation %d at (%d,%d): got %d want %d", o, x, y, got, want)
				}
			}
		}
	}
}

func TestExifOrientation(t *testing.T) {
	// SOI, APP1 "Exif", big-endian TIFF with one IFD entry: 0x0112 = 6
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 6, 0, 0, 0, 0, 0, 0}
	seg := append([]byte("Exif\x00\x00"), tiff...)
	b := []byte{0xFF, 0xD8, 0xFF, 0xE1, byte((len(seg) + 2) >> 8), byte(len(seg) + 2)}
	b = append(b, seg...)
	b = append(b, 0xFF, 0xDA)
	if o := exifOrientation(b); o != 6 {
		t.Fatalf("got %d, want 6", o)
	}
	if o := exifOrientation([]byte{0x89, 'P', 'N', 'G'}); o != 1 {
		t.Fatalf("non-JPEG: got %d, want 1", o)
	}
}

// Cells must tile the canvas exactly, with every cell at its photo's aspect
// when the canvas has the layout's natural aspect.
func TestCellsTile(t *testing.T) {
	aspects := []float64{1.5, 0.67, 1, 1.78, 0.8}
	l := engine.Solve(aspects).Layouts(1, 1)[0]
	W := 1000.0
	H := W / l.Tree.A
	area := 0.0
	for _, c := range Cells(l.Tree, 0, 0, W, H, nil) {
		area += c.W * c.H
		if d := math.Abs(c.W/c.H - aspects[c.Photo]); d > 1e-9 {
			t.Fatalf("photo %d cell aspect %.4f, want %.4f", c.Photo, c.W/c.H, aspects[c.Photo])
		}
	}
	if math.Abs(area-W*H) > 1e-6 {
		t.Fatalf("cells cover %.2f of %.2f", area, W*H)
	}
}
