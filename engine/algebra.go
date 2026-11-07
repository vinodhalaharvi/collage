package engine

import "math"

// Monoid is the "choose" half of a semiring: ⊕ with identity Zero.
// Laws: associative, commutative, Zero is the identity.
type Monoid[T any] struct {
	Zero   T
	Plus   func(a, b T) T
	IsZero func(T) bool
}

// Algebra is an F-algebra F T → T over a commutative monoid (T, ⊕).
// Its law is distributivity, which for a functor reads as bilinearity:
//
//	Alg(Split c (x⊕y) z) = Alg(Split c x z) ⊕ Alg(Split c y z)   (and on the right)
//	Alg(Split c 0 z)     = 0                                      (annihilation)
type Algebra[T any] struct {
	Monoid[T]
	Alg func(F[T]) T
}

// GradedAlgebra is an algebra whose Split may read the children's grades
// (their aspect ratios). Lift turns it into an ordinary Algebra.
type GradedAlgebra[T any] struct {
	Monoid[T]
	Alg func(n F[T], gL, gR float64) T // for a leaf, gL is the leaf's grade
}

// ---------- Constructor 1: Product  (S × T, componentwise) ----------

type Pair[A, B any] struct {
	A A
	B B
}

func Product[A, B any](x GradedAlgebra[A], y GradedAlgebra[B]) GradedAlgebra[Pair[A, B]] {
	fst := func(p Pair[A, B]) A { return p.A }
	snd := func(p Pair[A, B]) B { return p.B }
	return GradedAlgebra[Pair[A, B]]{
		Monoid: Monoid[Pair[A, B]]{
			Zero:   Pair[A, B]{x.Zero, y.Zero},
			Plus:   func(p, q Pair[A, B]) Pair[A, B] { return Pair[A, B]{x.Plus(p.A, q.A), y.Plus(p.B, q.B)} },
			IsZero: func(p Pair[A, B]) bool { return x.IsZero(p.A) && y.IsZero(p.B) },
		},
		Alg: func(n F[Pair[A, B]], gL, gR float64) Pair[A, B] {
			return Pair[A, B]{x.Alg(Fmap(fst, n), gL, gR), y.Alg(Fmap(snd, n), gL, gR)}
		},
	}
}

// ---------- Constructor 2: Lift  (the monoid semiring T[M]) ----------
//
// Grades are aspect buckets. A Vec is a finitely supported map grade → T,
// like a polynomial whose exponents live in the aspect monoid instead of ℕ.
// Splitting two Vecs is a convolution: grades combine through Aspect,
// coefficients combine through the base algebra.

type Grades struct {
	B        int
	lo, step float64
	center   []float64
	comb     [2][]int
}

// NewGrades buckets log-aspect uniformly over [1/maxAspect, maxAspect].
func NewGrades(b int, maxAspect float64) *Grades {
	lo, hi := -math.Log(maxAspect), math.Log(maxAspect)
	g := &Grades{B: b, lo: lo, step: (hi - lo) / float64(b-1)}
	g.center = make([]float64, b)
	for i := range b {
		g.center[i] = math.Exp(lo + float64(i)*g.step)
	}
	for c := range 2 {
		g.comb[c] = make([]int, b*b)
		for i := range b {
			for j := range b {
				g.comb[c][i*b+j] = g.Of(Aspect(Cut(c), g.center[i], g.center[j]))
			}
		}
	}
	return g
}

// Of returns the bucket of an aspect, or -1 when it falls outside the range.
func (g *Grades) Of(a float64) int {
	x := math.Round((math.Log(a) - g.lo) / g.step)
	if x < 0 || x >= float64(g.B) {
		return -1
	}
	return int(x)
}

func (g *Grades) Center(b int) float64 { return g.center[b] }

func (g *Grades) Combine(c Cut, b1, b2 int) int { return g.comb[c][b1*g.B+b2] }

type Vec[T any] struct {
	At []T   // dense coefficients; nil means the zero vector
	NZ []int // support, in insertion order
}

func Lift[T any](g GradedAlgebra[T], gr *Grades, leafGrade func(photo int) int) Algebra[Vec[T]] {
	add := func(v *Vec[T], b int, x T) {
		if g.IsZero(x) {
			return
		}
		if v.At == nil {
			v.At = make([]T, gr.B)
			for i := range v.At {
				v.At[i] = g.Zero
			}
		}
		if g.IsZero(v.At[b]) {
			v.NZ = append(v.NZ, b)
			v.At[b] = x
			return
		}
		v.At[b] = g.Plus(v.At[b], x)
	}
	plus := func(a, b Vec[T]) Vec[T] {
		if a.At == nil {
			return b
		}
		if b.At == nil {
			return a
		}
		out := Vec[T]{At: append([]T(nil), a.At...), NZ: append([]int(nil), a.NZ...)}
		for _, i := range b.NZ {
			add(&out, i, b.At[i])
		}
		return out
	}
	alg := func(n F[Vec[T]]) Vec[T] {
		var out Vec[T]
		if n.IsLeaf {
			if b := leafGrade(n.Photo); b >= 0 {
				add(&out, b, g.Alg(F[T]{IsLeaf: true, Photo: n.Photo}, gr.Center(b), 0))
			}
			return out
		}
		for _, b1 := range n.L.NZ {
			for _, b2 := range n.R.NZ {
				b := gr.Combine(n.Cut, b1, b2)
				if b < 0 {
					continue
				}
				x := g.Alg(F[T]{Cut: n.Cut, L: n.L.At[b1], R: n.R.At[b2]}, gr.Center(b1), gr.Center(b2))
				add(&out, b, x)
			}
		}
		return out
	}
	return Algebra[Vec[T]]{
		Monoid: Monoid[Vec[T]]{Zero: Vec[T]{}, Plus: plus, IsZero: func(v Vec[T]) bool { return len(v.NZ) == 0 }},
		Alg:    alg,
	}
}
