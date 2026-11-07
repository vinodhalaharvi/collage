package engine

// Coalgebra is a nondeterministic F-coalgebra S → List(F S):
// every way a state can be split into smaller states (the "projections").
type Coalgebra[S any] func(S) []F[S]

type Stats struct {
	States       int // distinct subproblems solved
	Alternatives int // F-nodes folded (edges of the DAG)
}

// Dyna is the dynamorphism: the hylomorphism
//
//	rec = ⊕ ∘ map(alg ∘ F rec) ∘ coalg
//
// memoised on the state, so each subproblem is solved exactly once.
// Correctness needs only the Algebra laws (distributivity = bilinearity).
func Dyna[S comparable, T any](a Algebra[T], coalg Coalgebra[S], goal S) (T, Stats) {
	memo := map[S]T{}
	var st Stats
	var rec func(S) T
	rec = func(s S) T {
		if v, ok := memo[s]; ok {
			return v
		}
		acc := a.Zero
		for _, node := range coalg(s) {
			st.Alternatives++
			acc = a.Plus(acc, a.Alg(Fmap(rec, node)))
		}
		memo[s] = acc
		st.States++
		return acc
	}
	v := rec(goal)
	return v, st
}

// Tree is the initial algebra μF: one concrete layout.
type Tree struct{ Node F[*Tree] }

// Cata folds a single tree with an algebra (the catamorphism).
func Cata[T any](alg func(F[T]) T, t *Tree) T {
	return alg(Fmap(func(c *Tree) T { return Cata(alg, c) }, t.Node))
}

// Unfold grows every tree the coalgebra allows from s, without sharing.
func Unfold[S any](coalg Coalgebra[S], s S) []*Tree {
	var out []*Tree
	for _, n := range coalg(s) {
		if n.IsLeaf {
			out = append(out, &Tree{F[*Tree]{IsLeaf: true, Photo: n.Photo}})
			continue
		}
		for _, l := range Unfold(coalg, n.L) {
			for _, r := range Unfold(coalg, n.R) {
				out = append(out, &Tree{F[*Tree]{Cut: n.Cut, L: l, R: r}})
			}
		}
	}
	return out
}

// BruteForce is the specification: ⊕ over every tree of its catamorphism
// ("sum over all paths of the product along each path"). Dyna must equal it.
func BruteForce[S any, T any](a Algebra[T], coalg Coalgebra[S], s S) (T, int) {
	trees := Unfold(coalg, s)
	acc := a.Zero
	for _, t := range trees {
		acc = a.Plus(acc, Cata(a.Alg, t))
	}
	return acc, len(trees)
}
