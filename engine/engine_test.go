package engine

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

func TestLaws(t *testing.T) {
	for _, r := range AllLaws(20000, 1) {
		if !r.OK {
			t.Errorf("%s: %s FAILED", r.Algebra, r.Law)
		}
	}
}

func TestDynaEqualsBruteForce(t *testing.T) {
	for n := 1; n <= 6; n++ {
		for seed := int64(0); seed < 5; seed++ {
			c := VerifyDyna(n, seed)
			if !c.OK {
				t.Fatalf("n=%d seed=%d: dyna != brute force", n, seed)
			}
			if seed == 0 {
				t.Logf("n=%d trees=%d states=%d alternatives=%d", n, c.Trees, c.States, c.Alternatives)
			}
		}
	}
}

func TestSolveSpeed(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	for n := 5; n <= 9; n++ {
		a := make([]float64, n)
		for i := range a {
			a[i] = math.Exp((r.Float64()*2 - 1) * math.Log(2))
		}
		t0 := time.Now()
		c := Solve(a)
		el := time.Since(t0)
		_, total := c.Histogram()
		best := c.Layouts(1, 1)[0]
		t.Logf("n=%d %v states=%d alts=%d layouts=%.0f best share=%.3f crop=%.3f", n, el, c.Stats.States, c.Stats.Alternatives, total, best.MinShare, best.Crop)
	}
}
