// Package engine is a dynamic-programming collage solver written as
// recursion schemes over a layout functor.
//
//	F X  =  Leaf Photo  |  Split Cut X X          (the shape of one layout step)
//	coalgebra  S → List(F S)                      (how a problem shrinks: the projections)
//	algebra    F T → T, bilinear over (T, ⊕)      (how answers combine: the semiring)
//	dyna       ⊕ ∘ map(alg ∘ F rec) ∘ coalg, memoised   (dynamic programming)
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

// F is the layout functor F X = Leaf Photo | Split Cut X X.
type F[X any] struct {
	IsLeaf bool
	Photo  int
	Cut    Cut
	L, R   X
}

// Fmap is F's action on arrows: apply f to every child, keep the shape.
func Fmap[X, Y any](f func(X) Y, n F[X]) F[Y] {
	if n.IsLeaf {
		return F[Y]{IsLeaf: true, Photo: n.Photo}
	}
	return F[Y]{Cut: n.Cut, L: f(n.L), R: f(n.R)}
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
