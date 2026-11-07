// Package engine is the collage solver: a dynamic program over guillotine
// layouts, written with github.com/vinodhalaharvi/dyna.
//
//	F X       = Leaf photo | Split cut X X        one layout step
//	coalgebra   {photos} → every way to split them in two
//	algebra     Lift(Count × TopK) graded by aspect ratio
package engine

// Cut labels a binary split: H puts children side by side, V stacks them.
type Cut uint8

const (
	H Cut = iota
	V
)

func (c Cut) String() string {
	if c == H {
		return "H"
	}
	return "V"
}

// Aspect is the aspect monoid on layouts: side by side at equal height the
// widths add; stacked at equal width the heights (reciprocals) add.
func Aspect(c Cut, aL, aR float64) float64 {
	if c == H {
		return aL + aR
	}
	return 1 / (1/aL + 1/aR)
}

// FracL is the share of the parent's area that the left child receives.
func FracL(c Cut, aL, aR float64) float64 {
	if c == H {
		return aL / (aL + aR)
	}
	hL, hR := 1/aL, 1/aR
	return hL / (hL + hR)
}
