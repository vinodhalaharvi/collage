Install with Homebrew:
```sh
brew install vinodhalaharvi/tap/collage
```

- `collage *.jpg` makes a collage of up to 10 photos in any shape (`-ratio story`, `16:9`, `3:1`…), with `-gutter`, `-bg` and `-suggest N` for structurally different layouts.
- `collage serve` opens the interactive editor in your browser. The solver runs there as WebAssembly, so it works offline.
- `collage laws` property-tests the semiring laws and checks the dynamic program against brute force.

Every guillotine layout is searched by one dynamic program (a memoised hylomorphism over the layout functor), choosing the layout where the smallest photo is largest and the crop is smallest. With 9 photos that is over 500 million layouts in under a second.

JPEG, PNG and GIF everywhere; HEIC and WebP on macOS via `sips`. EXIF rotation is respected.

Binaries below are for macOS (Apple Silicon and Intel) and Linux (x86-64 and ARM64). The macOS binaries are not notarised; if Gatekeeper blocks one, use Homebrew or run `xattr -d com.apple.quarantine collage`.
