package engine

import (
	"fmt"
	"math"
	"math/bits"
	"sort"

	"github.com/vinodhalaharvi/dyna"
)

// ---------- The functor's labels and the coalgebra ----------

// Step labels one layout step: a leaf holding a photo, or a cut with two children.
type Step struct {
	Leaf  bool
	Photo int
	Cut   Cut
}

// Splits is the collage coalgebra on bitmask states. A single photo is a leaf;
// otherwise every split into A ∪ B (A holding the lowest photo, so mirror
// images are not double-counted) can be cut side by side or stacked.
func Splits() dyna.Coalgebra[uint32, Step] {
	return func(m uint32) []dyna.Node[Step, uint32] {
		if m&(m-1) == 0 {
			return []dyna.Node[Step, uint32]{{Label: Step{Leaf: true, Photo: bits.TrailingZeros32(m)}}}
		}
		low := m & -m
		rest := m ^ low
		var out []dyna.Node[Step, uint32]
		for sub := rest; ; sub = (sub - 1) & rest {
			if a := low | sub; a != m {
				b := m ^ a
				out = append(out,
					dyna.Node[Step, uint32]{Label: Step{Cut: H}, Kids: []uint32{a, b}},
					dyna.Node[Step, uint32]{Label: Step{Cut: V}, Kids: []uint32{a, b}})
			}
			if sub == 0 {
				break
			}
		}
		return out
	}
}

// ---------- Grades: aspect-ratio buckets ----------

// Grades buckets log-aspect uniformly over [1/maxAspect, maxAspect]; a cut
// combines buckets through the aspect monoid.
type Grades struct {
	B        int
	lo, step float64
	center   []float64
	comb     [2][]int
}

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

// Grading turns the buckets into dyna's grading for these photos.
func (g *Grades) Grading(aspects []float64) dyna.Grading[Step] {
	return dyna.Grading[Step]{N: g.B, Value: g.Center, Of: func(l Step, kids []int) int {
		if l.Leaf {
			return g.Of(aspects[l.Photo])
		}
		return g.comb[l.Cut][kids[0]*g.B+kids[1]]
	}}
}

// ---------- The algebra: Count × TopK, lifted by aspect ----------

// K is how many layouts are kept per subproblem and aspect.
const K = 3

// shareScore is the area share of the smallest photo. Maximising the minimum
// is the (max, min) "widest path" semiring; each side is scaled by its share
// of the area, which is monotone, so TopK distributes over ⊕.
func shareScore(l Step, kids []float64, _ float64, grades []float64) float64 {
	if l.Leaf {
		return 1
	}
	f := FracL(l.Cut, grades[0], grades[1])
	return math.Min(kids[0]*f, kids[1]*(1-f))
}

type Cell = dyna.Pair[float64, dyna.Ranked[Step]]

type Collage struct {
	Aspects []float64
	Grades  *Grades
	Root    dyna.Vec[Cell]
	Stats   dyna.Stats
}

const (
	Buckets   = 64
	MaxAspect = 12.0
	// FitWeight trades crop against balance when ranking: rank = minShare · fit^FitWeight.
	FitWeight = 8.0
)

func NewAlgebra(aspects []float64, gr *Grades) dyna.Algebra[Step, dyna.Vec[Cell]] {
	return dyna.Lift(dyna.Product(dyna.Count[Step](), dyna.TopK(K, shareScore)), gr.Grading(aspects))
}

func Solve(aspects []float64) *Collage {
	gr := NewGrades(Buckets, MaxAspect)
	full := uint32(1)<<len(aspects) - 1
	root, st := dyna.Dyna(NewAlgebra(aspects, gr), Splits(), full)
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
func (c *Collage) exact(e *dyna.Entry[Step]) (*Node, float64) {
	if e.Label.Leaf {
		return &Node{P: e.Label.Photo, A: c.Aspects[e.Label.Photo]}, 1
	}
	cut := e.Label.Cut
	l, sL := c.exact(e.Kids[0])
	r, sR := c.exact(e.Kids[1])
	f := FracL(cut, l.A, r.A)
	return &Node{P: -1, C: cut.String(), A: Aspect(cut, l.A, r.A), L: l, R: r}, math.Min(sL*f, sR*(1-f))
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
		for _, e := range c.Root.At[b].B {
			n, share := c.exact(e)
			fit := math.Min(n.A/canvas, canvas/n.A)
			all = append(all, Layout{Tree: n, Aspect: n.A, MinShare: share, Crop: 1 - fit, Rank: share * math.Pow(fit, FitWeight)})
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Rank > all[j].Rank })
	// Suggestions should differ at a glance, not by a swap deep in one column:
	// keep the best layout for each (hero photo, first cut, photos left of it).
	var out []Layout
	seen := map[string]bool{}
	for _, l := range all {
		if s := gist(l.Tree); !seen[s] {
			seen[s] = true
			out = append(out, l)
			if len(out) == k {
				break
			}
		}
	}
	return out
}

// gist names what a viewer notices first: the largest photo, the direction
// of the first cut, and how many photos sit on its left (or top) side.
func gist(n *Node) string {
	hero, best := -1, -1.0
	var walk func(*Node, float64)
	walk = func(m *Node, share float64) {
		if m.P >= 0 {
			if share > best {
				hero, best = m.P, share
			}
			return
		}
		f := FracL(map[string]Cut{"H": H, "V": V}[m.C], m.L.A, m.R.A)
		walk(m.L, share*f)
		walk(m.R, share*(1-f))
	}
	walk(n, 1)
	return fmt.Sprintf("%d/%s/%d", hero, n.C, leaves(n.L))
}

func leaves(n *Node) int {
	if n == nil {
		return 0
	}
	if n.P >= 0 {
		return 1
	}
	return leaves(n.L) + leaves(n.R)
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
