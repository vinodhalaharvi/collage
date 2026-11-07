package render

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/vinodhalaharvi/collage/engine"
)

// Rect is a cell of the collage in canvas pixels, before gutters.
type Rect struct {
	Photo      int
	X, Y, W, H float64
}

// Cells unfolds a layout tree into rectangles: side-by-side children share
// the width in proportion to their aspects, stacked children share the height
// in proportion to the reciprocals (the aspect monoid, read backwards).
func Cells(n *engine.Node, x, y, w, h float64, out []Rect) []Rect {
	if n.P >= 0 {
		return append(out, Rect{n.P, x, y, w, h})
	}
	if n.C == "H" {
		wl := w * n.L.A / (n.L.A + n.R.A)
		out = Cells(n.L, x, y, wl, h, out)
		return Cells(n.R, x+wl, y, w-wl, h, out)
	}
	il, ir := 1/n.L.A, 1/n.R.A
	hl := h * il / (il + ir)
	out = Cells(n.L, x, y, w, hl, out)
	return Cells(n.R, x, y+hl, w, h-hl, out)
}

type Options struct {
	Width, Height int
	Gutter        int
	Background    color.RGBA
}

// Render draws the layout; photos are decoded one at a time to bound memory.
func Render(tree *engine.Node, photos []Photo, o Options) (*image.RGBA, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, o.Width, o.Height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{o.Background}, image.Point{}, draw.Src)
	g := float64(o.Gutter)
	for _, c := range Cells(tree, g/2, g/2, float64(o.Width)-g, float64(o.Height)-g, nil) {
		x0, y0 := int(math.Round(c.X+g/2)), int(math.Round(c.Y+g/2))
		x1, y1 := int(math.Round(c.X+c.W-g/2)), int(math.Round(c.Y+c.H-g/2))
		if x1 <= x0 || y1 <= y0 {
			continue
		}
		tile, err := cover(photos[c.Photo], x1-x0, y1-y0)
		if err != nil {
			return nil, err
		}
		draw.Draw(canvas, image.Rect(x0, y0, x1, y1), tile, image.Point{}, draw.Src)
	}
	return canvas, nil
}

// cover scales and centre-crops a photo to exactly tw×th upright pixels.
func cover(p Photo, tw, th int) (*image.RGBA, error) {
	img, err := p.Load()
	if err != nil {
		return nil, err
	}
	// crop rectangle in upright coordinates
	s := math.Max(float64(tw)/float64(p.W), float64(th)/float64(p.H))
	cw, ch := float64(tw)/s, float64(th)/s
	cx, cy := (float64(p.W)-cw)/2, (float64(p.H)-ch)/2
	// map it back to the stored (sensor) orientation
	sx0, sy0 := toSource(p, cx, cy)
	sx1, sy1 := toSource(p, cx+cw, cy+ch)
	b := img.Bounds()
	src := image.Rect(
		b.Min.X+int(math.Floor(math.Min(sx0, sx1))), b.Min.Y+int(math.Floor(math.Min(sy0, sy1))),
		b.Min.X+int(math.Ceil(math.Max(sx0, sx1))), b.Min.Y+int(math.Ceil(math.Max(sy0, sy1)))).Intersect(b)
	region := image.NewRGBA(image.Rect(0, 0, src.Dx(), src.Dy()))
	draw.Draw(region, region.Bounds(), img, src.Min, draw.Src)
	dw, dh := tw, th
	if p.Orient >= 5 {
		dw, dh = th, tw
	}
	return orient(resample(region, dw, dh), p.Orient), nil
}

// toSource maps an upright point to the stored image's coordinates.
func toSource(p Photo, x, y float64) (float64, float64) {
	W, H := float64(p.W), float64(p.H) // upright size
	switch p.Orient {
	case 2:
		return W - x, y
	case 3:
		return W - x, H - y
	case 4:
		return x, H - y
	case 5:
		return y, x
	case 6:
		return y, W - x
	case 7:
		return H - y, W - x
	case 8:
		return H - y, x
	}
	return x, y
}

// orient rotates/flips a stored-orientation image to upright.
func orient(m *image.RGBA, o int) *image.RGBA {
	if o <= 1 {
		return m
	}
	sw, sh := m.Bounds().Dx(), m.Bounds().Dy()
	dw, dh := sw, sh
	if o >= 5 {
		dw, dh = sh, sw
	}
	out := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			var u, v int
			switch o {
			case 2:
				u, v = sw-1-x, y
			case 3:
				u, v = sw-1-x, sh-1-y
			case 4:
				u, v = x, sh-1-y
			case 5:
				u, v = y, x
			case 6:
				u, v = y, sh-1-x
			case 7:
				u, v = sw-1-y, sh-1-x
			case 8:
				u, v = sw-1-y, x
			}
			si, di := m.PixOffset(u, v), out.PixOffset(x, y)
			copy(out.Pix[di:di+4], m.Pix[si:si+4])
		}
	}
	return out
}

// resample scales with an area-average (box) filter when shrinking and
// bilinear interpolation when enlarging.
func resample(src *image.RGBA, dw, dh int) *image.RGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	if sw == 0 || sh == 0 {
		return dst
	}
	fx, fy := float64(sw)/float64(dw), float64(sh)/float64(dh)
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			var r, g, b, a float64
			if fx >= 1 && fy >= 1 {
				x0, x1 := int(float64(x)*fx), max(int(float64(x)*fx)+1, int(float64(x+1)*fx))
				y0, y1 := int(float64(y)*fy), max(int(float64(y)*fy)+1, int(float64(y+1)*fy))
				x1, y1 = min(x1, sw), min(y1, sh)
				n := 0.0
				for v := y0; v < y1; v++ {
					i := src.PixOffset(x0, v)
					for u := x0; u < x1; u++ {
						r += float64(src.Pix[i])
						g += float64(src.Pix[i+1])
						b += float64(src.Pix[i+2])
						a += float64(src.Pix[i+3])
						i += 4
						n++
					}
				}
				r, g, b, a = r/n, g/n, b/n, a/n
			} else {
				u := math.Max(0, (float64(x)+0.5)*fx-0.5)
				v := math.Max(0, (float64(y)+0.5)*fy-0.5)
				u0, v0 := int(u), int(v)
				u1, v1 := min(u0+1, sw-1), min(v0+1, sh-1)
				du, dv := u-float64(u0), v-float64(v0)
				for k := 0; k < 4; k++ {
					p00 := float64(src.Pix[src.PixOffset(u0, v0)+k])
					p10 := float64(src.Pix[src.PixOffset(u1, v0)+k])
					p01 := float64(src.Pix[src.PixOffset(u0, v1)+k])
					p11 := float64(src.Pix[src.PixOffset(u1, v1)+k])
					val := (p00*(1-du)+p10*du)*(1-dv) + (p01*(1-du)+p11*du)*dv
					switch k {
					case 0:
						r = val
					case 1:
						g = val
					case 2:
						b = val
					case 3:
						a = val
					}
				}
			}
			i := dst.PixOffset(x, y)
			dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = uint8(r+0.5), uint8(g+0.5), uint8(b+0.5), uint8(a+0.5)
		}
	}
	return dst
}
