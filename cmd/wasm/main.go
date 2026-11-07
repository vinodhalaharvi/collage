//go:build js && wasm

// Command wasm exposes the collage dynamorphism to the browser.
package main

import (
	"encoding/json"
	"syscall/js"
	"time"

	"github.com/vinodhalaharvi/collage/engine"
)

var cur *engine.Collage

func toJS(v any) any {
	b, _ := json.Marshal(v)
	return string(b)
}

func main() {
	js.Global().Set("dpSolve", js.FuncOf(func(_ js.Value, args []js.Value) any {
		arr := args[0]
		aspects := make([]float64, arr.Length())
		for i := range aspects {
			aspects[i] = arr.Index(i).Float()
		}
		t0 := time.Now()
		cur = engine.Solve(aspects)
		ms := float64(time.Since(t0).Microseconds()) / 1000
		_, total := cur.Histogram()
		return toJS(map[string]any{"ms": ms, "states": cur.Stats.States, "alternatives": cur.Stats.Alternatives, "layouts": total})
	}))
	js.Global().Set("dpLayouts", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return toJS(cur.Layouts(args[0].Float(), args[1].Int()))
	}))
	js.Global().Set("dpHistogram", js.FuncOf(func(_ js.Value, _ []js.Value) any {
		bars, total := cur.Histogram()
		return toJS(map[string]any{"bars": bars, "total": total})
	}))
	js.Global().Set("dpLaws", js.FuncOf(func(_ js.Value, _ []js.Value) any {
		t0 := time.Now()
		laws := engine.AllLaws(2000, time.Now().UnixNano())
		check := engine.VerifyDyna(5, time.Now().UnixNano())
		ms := float64(time.Since(t0).Microseconds()) / 1000
		return toJS(map[string]any{"laws": laws, "dyna": check, "ms": ms})
	}))
	js.Global().Call("dpReady")
	select {}
}
