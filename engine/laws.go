package engine

import (
	"math"
	"math/rand"
)

type LawResult struct {
	Algebra string `json:"algebra"`
	Law     string `json:"law"`
	OK      bool   `json:"ok"`
	Trials  int    `json:"trials"`
}

// CheckLaws property-tests the semiring laws of a graded algebra on random
// elements: ⊕ is a commutative monoid, and Split is bilinear and annihilated by 0.
func CheckLaws[T any](name string, g GradedAlgebra[T], gen func(*rand.Rand) T, eq func(a, b T) bool, trials int, r *rand.Rand) []LawResult {
	split := func(c Cut, l, rr T, gL, gR float64) T { return g.Alg(F[T]{Cut: c, L: l, R: rr}, gL, gR) }
	laws := []struct {
		name string
		ok   func(x, y, z T, c Cut, gL, gR float64) bool
	}{
		{"⊕ associative", func(x, y, z T, _ Cut, _, _ float64) bool { return eq(g.Plus(g.Plus(x, y), z), g.Plus(x, g.Plus(y, z))) }},
		{"⊕ commutative", func(x, y, _ T, _ Cut, _, _ float64) bool { return eq(g.Plus(x, y), g.Plus(y, x)) }},
		{"0 is ⊕-identity", func(x, _, _ T, _ Cut, _, _ float64) bool { return eq(g.Plus(x, g.Zero), x) }},
		{"distributes on left", func(x, y, z T, c Cut, gL, gR float64) bool {
			return eq(split(c, g.Plus(x, y), z, gL, gR), g.Plus(split(c, x, z, gL, gR), split(c, y, z, gL, gR)))
		}},
		{"distributes on right", func(x, y, z T, c Cut, gL, gR float64) bool {
			return eq(split(c, z, g.Plus(x, y), gL, gR), g.Plus(split(c, z, x, gL, gR), split(c, z, y, gL, gR)))
		}},
		{"0 annihilates", func(x, _, _ T, c Cut, gL, gR float64) bool {
			return g.IsZero(split(c, g.Zero, x, gL, gR)) && g.IsZero(split(c, x, g.Zero, gL, gR))
		}},
	}
	var out []LawResult
	for _, l := range laws {
		ok := true
		for i := 0; i < trials && ok; i++ {
			c := Cut(r.Intn(2))
			gL, gR := 0.2+r.Float64()*5, 0.2+r.Float64()*5
			ok = l.ok(gen(r), gen(r), gen(r), c, gL, gR)
		}
		out = append(out, LawResult{name, l.name, ok, trials})
	}
	return out
}

func GenCount(r *rand.Rand) float64 { return float64(r.Intn(6)) }
func EqCount(a, b float64) bool     { return a == b }

func GenTopK(r *rand.Rand) TopK {
	var t TopK
	t.N = r.Intn(K + 1)
	s := 1.0
	for i := 0; i < t.N; i++ {
		s *= 0.3 + 0.7*r.Float64()
		if r.Intn(4) == 0 && i > 0 {
			s = t.E[i-1].Score // ties on purpose
		}
		t.E[i] = &Entry{Score: s, Leaf: true, Photo: i}
	}
	return t
}

func EqTopK(a, b TopK) bool {
	if a.N != b.N {
		return false
	}
	for i := 0; i < a.N; i++ {
		if math.Abs(a.E[i].Score-b.E[i].Score) > 1e-12 {
			return false
		}
	}
	return true
}

// AllLaws checks Count, TopK and their Product.
func AllLaws(trials int, seed int64) []LawResult {
	r := rand.New(rand.NewSource(seed))
	var out []LawResult
	out = append(out, CheckLaws("Count (+,×)", CountAlgebra(), GenCount, EqCount, trials, r)...)
	out = append(out, CheckLaws("TopK₃ (max,min)", TopKAlgebra(), GenTopK, EqTopK, trials, r)...)
	out = append(out, CheckLaws("Count × TopK₃", Product(CountAlgebra(), TopKAlgebra()),
		func(r *rand.Rand) Cell { return Cell{GenCount(r), GenTopK(r)} },
		func(a, b Cell) bool { return EqCount(a.A, b.A) && EqTopK(a.B, b.B) }, trials, r)...)
	return out
}

type DynaCheck struct {
	Photos       int  `json:"photos"`
	Trees        int  `json:"trees"`
	States       int  `json:"states"`
	Alternatives int  `json:"alternatives"`
	OK           bool `json:"ok"`
}

// VerifyDyna compares the memoised dynamorphism with the brute-force
// specification (fold every tree, ⊕ the results) on random photos.
func VerifyDyna(n int, seed int64) DynaCheck {
	r := rand.New(rand.NewSource(seed))
	aspects := make([]float64, n)
	for i := range aspects {
		aspects[i] = math.Exp((r.Float64()*2 - 1) * math.Log(2.5))
	}
	gr := NewGrades(Buckets, MaxAspect)
	alg := NewAlgebra(aspects, gr)
	full := uint32(1)<<n - 1
	dp, st := Dyna(alg, Splits(), full)
	bf, trees := BruteForce(alg, Splits(), full)
	ok := len(dp.NZ) == len(bf.NZ)
	for b := 0; ok && b < gr.B; b++ {
		if dp.At == nil || bf.At == nil {
			ok = dp.At == nil && bf.At == nil
			break
		}
		ok = math.Abs(dp.At[b].A-bf.At[b].A) < 1e-6 && EqTopK(dp.At[b].B, bf.At[b].B)
	}
	return DynaCheck{n, trees, st.States, st.Alternatives, ok}
}
