# collage

Arrange photos into a gap-free collage of any shape, from the command line or in your browser.

```sh
brew install vinodhalaharvi/tap/collage

collage ~/Pictures/trip/*.jpg                       # square collage.jpg
collage -ratio story -o story.jpg a.jpg b.jpg c.jpg  # 9:16
collage -ratio 3:1 -gutter 0 -suggest 3 *.png        # three different banners
collage serve                                        # interactive editor
```

Every arrangement is searched, not guessed. With 9 photos there are over 500 million guillotine layouts (every split is a straight cut through the whole region). `collage` finds the one where the smallest photo is as large as possible and the crop to your canvas is as small as possible, in well under a second.

## Flags

| Flag | Meaning | Default |
|---|---|---|
| `-o FILE` | output, `.jpg` or `.png` | `collage.jpg` |
| `-ratio R` | `16:9`, `1.5`, or `square`, `portrait` (4:5), `story` (9:16), `landscape` (16:9), `banner` (3:1) | `square` |
| `-size PX` | longer side of the canvas | `2400` |
| `-gutter PX` | space between photos | `12` |
| `-bg COLOR` | background `#rrggbb` | `#ffffff` |
| `-suggest N` | write N structurally different layouts | `1` |

Up to 10 photos. JPEG, PNG and GIF work everywhere. On macOS, HEIC (the iPhone default) and WebP are converted on the fly with the built-in `sips`. Phone photos are turned upright from their EXIF orientation.

## How it works

The solver is one dynamic program, written as recursion schemes:

```
F X   = Leaf photo | Split cut X X                 one layout step (a functor)
coalg : {photos} → [Leaf p | Split H A B | Split V A B]   how a problem shrinks
alg   : F T → T,  bilinear over (T, ⊕)              how answers combine (a semiring)
dyna  = ⊕ ∘ map(alg ∘ F dyna) ∘ coalg, memoised     dynamic programming
```

Side-by-side photos have aspect ratios that add (a = a₁ + a₂); stacked photos add as reciprocals (1/a = 1/a₁ + 1/a₂). The state is `(subset of photos, aspect bucket)`, so the answer is indexed by aspect ratio and resizing the canvas is a lookup, not a re-solve.

The algebra is built from parts:

- `Count` (+, ×) counts layouts.
- `TopK₃` (max, min) keeps the three layouts whose smallest photo is largest.
- `Product` runs both in one pass; `Lift` (the monoid-semiring construction) grades them by aspect ratio.

The DP is only correct if the algebra obeys the semiring laws, distributivity above all. `collage laws` checks them with random trials and compares the DP against brute force:

```
$ collage laws
  Count × TopK₃
    distributes on left    holds
    ...
    6 photos   30,240 layouts   608 steps over 63 subproblems  equal
```

Aspect ratios are bucketed (64 buckets over 1:12 to 12:1), so the search is exact for the bucketed problem; the chosen layout is then re-measured with each photo's true shape.

## Building from source

```sh
go build ./cmd/collage          # web/engine.wasm is committed
make wasm                       # rebuild the browser engine after changing engine/
make test
```

## License

MIT
