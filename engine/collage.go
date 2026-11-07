package engine

import (
	"math"
	"math/bits"
	"sort"
)

// ---------- The coalgebra: a set of photos shrinks into two halves ----------

// Splits is the collage coalgebra on bitmask states. A single photo is a leaf;
// otherwise every split into A ∪ B (A holding the lowest photo, so mirror
// images are not double-counted) can be cut side by side or stacked.
func Splits() Coalgebra[uint32] {
	return func(m uint32) []F[uint32] {
		if m&(m-1) == 0 {
			return []F[uint32]{{IsLeaf: true, Photo: bits.TrailingZeros32(m)}}
		}
		low := m & -m
		rest := m ^ low
		var out []F[uint32]
		for sub := rest; ; sub = (sub - 1) & rest {
			if a := low | sub; a != m {
				b := m ^ a
				out = append(out, F[uint32]{Cut: H, L: a, R: b}, F[uint32]{Cut: V, L: a, R: b})
			}
			if sub == 0 {
				break
			}
		}
		return out
	}
}

// ---------- Algebra 1: Count  (+, ×) ----------

func CountAlgebra() GradedAlgebra[float64] {
	return GradedAlgebra[float64]{
		Monoid: Monoid[float64]{Zero: 0, Plus: func(a, b float64) float64 { return a + b }, IsZero: func(x float64) bool { return x == 0 }},
		Alg: func(n F[float64], _, _ float64) float64 {
			if n.IsLeaf {
				return 1
			}
			return n.L * n.R
		},
	}
}

// ---------- Algebra 2: TopK over the bottleneck score  (top-k of (max, min)) ----------
//
// Score = share of the canvas area given to the smallest photo. Maximising the
// minimum is the (max, min) "widest path" semiring; ⊗ scales each side by its
// area share and takes the min, which is monotone, hence bilinear under top-k.

const K = 3

type Entry struct {
	Score float64
	Leaf  bool
	Photo int
	Cut   Cut
	L, R  *Entry
}

type TopK struct {
	N int
	E [K]*Entry
}

func TopKAlgebra() GradedAlgebra[TopK] {
	plus := func(a, b TopK) TopK {
		var out TopK
		i, j := 0, 0
		for out.N < K && (i < a.N || j < b.N) {
			if j >= b.N || (i < a.N && a.E[i].Score >= b.E[j].Score) {
				out.E[out.N] = a.E[i]
				i++
			} else {
				out.E[out.N] = b.E[j]
				j++
			}
			out.N++
		}
		return out
	}
	alg := func(n F[TopK], gL, gR float64) TopK {
		if n.IsLeaf {
			return TopK{N: 1, E: [K]*Entry{{Score: 1, Leaf: true, Photo: n.Photo}}}
		}
		f := FracL(n.Cut, gL, gR)
		var sc [K * K]float64
		var ix [K * K][2]int
		m := 0
		for i := 0; i < n.L.N; i++ {
			for j := 0; j < n.R.N; j++ {
				sc[m] = math.Min(n.L.E[i].Score*f, n.R.E[j].Score*(1-f))
				ix[m] = [2]int{i, j}
				m++
			}
		}
		var out TopK
		var used [K * K]bool
		for out.N < K && out.N < m {
			best := -1
			for q := 0; q < m; q++ {
				if !used[q] && (best < 0 || sc[q] > sc[best]) {
					best = q
				}
			}
			used[best] = true
			out.E[out.N] = &Entry{Score: sc[best], Cut: n.Cut, L: n.L.E[ix[best][0]], R: n.R.E[ix[best][1]]}
			out.N++
		}
		return out
	}
	return GradedAlgebra[TopK]{
		Monoid: Monoid[TopK]{Zero: TopK{}, Plus: plus, IsZero: func(t TopK) bool { return t.N == 0 }},
		Alg:    alg,
	}
}

// ---------- The solver: one dynamorphism, algebra = Lift(Count × TopK) ----------

type Cell = Pair[float64, TopK]

type Collage struct {
	Aspects []float64
	Grades  *Grades
	Root    Vec[Cell]
	Stats   Stats
}

const (
	Buckets   = 64
	MaxAspect = 12.0
	// FitWeight trades crop against balance when ranking: rank = minShare · fit^FitWeight.
	FitWeight = 8.0
)

func NewAlgebra(aspects []float64, gr *Grades) Algebra[Vec[Cell]] {
	return Lift(Product(CountAlgebra(), TopKAlgebra()), gr, func(p int) int { return gr.Of(aspects[p]) })
}

func Solve(aspects []float64) *Collage {
	gr := NewGrades(Buckets, MaxAspect)
	full := uint32(1)<<len(aspects) - 1
	root, st := Dyna(NewAlgebra(aspects, gr), Splits(), full)
	return &Collage{Aspects: aspects, Grades: gr, Root: root, Stats: st}
}

// ---------- Reading answers out ----------

// Node is a layout tree evaluated with the photos' exact aspect ratios.
type Node struct {
	P int     `json:"p"` // photo index, -1 for a split
	C string  `json:"c,omitempty"`
	A float64 `json:"a"` // natural aspect of this sub-collage
	L *Node   `json:"l,omitempty"`
	R *Node   `json:"r,omitempty"`
}

// exact re-folds a witness tree with the real aspects (not bucket centres).
func (c *Collage) exact(e *Entry) (*Node, float64) {
	if e.Leaf {
		return &Node{P: e.Photo, A: c.Aspects[e.Photo]}, 1
	}
	l, sL := c.exact(e.L)
	r, sR := c.exact(e.R)
	f := FracL(e.Cut, l.A, r.A)
	return &Node{P: -1, C: e.Cut.String(), A: Aspect(e.Cut, l.A, r.A), L: l, R: r}, math.Min(sL*f, sR*(1-f))
}

type Layout struct {
	Tree     *Node   `json:"tree"`
	Aspect   float64 `json:"aspect"`
	MinShare float64 `json:"minShare"`
	Crop     float64 `json:"crop"`
	Rank     float64 `json:"rank"`
}

// Layouts ranks every stored witness for a canvas of the given aspect.
// No re-solve: the DP table is indexed by aspect, so resizing is a lookup.
func (c *Collage) Layouts(canvas float64, k int) []Layout {
	var all []Layout
	for _, b := range c.Root.NZ {
		t := c.Root.At[b].B
		for i := 0; i < t.N; i++ {
			n, share := c.exact(t.E[i])
			fit := math.Min(n.A/canvas, canvas/n.A)
			all = append(all, Layout{Tree: n, Aspect: n.A, MinShare: share, Crop: 1 - fit, Rank: share * math.Pow(fit, FitWeight)})
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Rank > all[j].Rank })
	// Suggestions should differ in structure, not just swap photos:
	// keep the best layout of each silhouette (tree shape, photos erased).
	var out []Layout
	seen := map[string]bool{}
	for _, l := range all {
		if s := silhouette(l.Tree); !seen[s] {
			seen[s] = true
			out = append(out, l)
			if len(out) == k {
				break
			}
		}
	}
	return out
}

func silhouette(n *Node) string {
	if n.P >= 0 {
		return "."
	}
	return n.C + "(" + silhouette(n.L) + silhouette(n.R) + ")"
}

type Bar struct {
	Aspect float64 `json:"aspect"`
	Count  float64 `json:"count"`
}

// Histogram is the Count component of the graded root vector: how many
// layouts exist at each aspect ratio.
func (c *Collage) Histogram() ([]Bar, float64) {
	var bars []Bar
	total := 0.0
	for _, b := range c.Root.NZ {
		n := c.Root.At[b].A
		bars = append(bars, Bar{Aspect: c.Grades.Center(b), Count: n})
		total += n
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].Aspect < bars[j].Aspect })
	return bars, total
}
