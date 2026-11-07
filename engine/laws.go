package engine

import (
	"math"
	"math/rand"

	"github.com/vinodhalaharvi/dyna"
)

type LawResult = dyna.LawResult

// AllLaws checks Count, TopK and their Product on collage's split nodes.
func AllLaws(trials int, seed int64) []LawResult {
	r := rand.New(rand.NewSource(seed))
	splits := []Step{{Cut: H}, {Cut: V}}
	two := func(Step) int { return 2 }
	genCount := func(r *rand.Rand) float64 { return float64(r.Intn(6)) }
	eqCount := func(a, b float64) bool { return a == b }
	genTop := dyna.GenRanked(K, Step{Leaf: true})
	var out []LawResult
	out = append(out, dyna.CheckLaws(dyna.Count[Step](), dyna.LawSpec[Step, float64]{
		Name: "Count (+,×)", Labels: splits, Arity: two, Gen: genCount, Eq: eqCount}, trials, r)...)
	out = append(out, dyna.CheckLaws(dyna.TopK(K, shareScore), dyna.LawSpec[Step, dyna.Ranked[Step]]{
		Name: "TopK₃ (max,min)", Labels: splits, Arity: two, Gen: genTop, Eq: dyna.RankedScoresEqual[Step]}, trials, r)...)
	out = append(out, dyna.CheckLaws(dyna.Product(dyna.Count[Step](), dyna.TopK(K, shareScore)), dyna.LawSpec[Step, Cell]{
		Name: "Count × TopK₃", Labels: splits, Arity: two,
		Gen: func(r *rand.Rand) Cell { return Cell{A: genCount(r), B: genTop(r)} },
		Eq:  func(a, b Cell) bool { return eqCount(a.A, b.A) && dyna.RankedScoresEqual(a.B, b.B) }}, trials, r)...)
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
	dp, st := dyna.Dyna(alg, Splits(), full)
	bf, trees := dyna.BruteForce(alg, Splits(), full)
	ok := len(dp.NZ) == len(bf.NZ)
	for b := 0; ok && b < gr.B; b++ {
		x, y := dp.Get(b, Cell{}), bf.Get(b, Cell{})
		ok = math.Abs(x.A-y.A) < 1e-6 && dyna.RankedScoresEqual(x.B, y.B)
	}
	return DynaCheck{n, trees, st.States, st.Alternatives, ok}
}
