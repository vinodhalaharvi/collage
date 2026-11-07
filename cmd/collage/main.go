// Command collage arranges photos into a gap-free collage of any shape.
// Every guillotine layout is searched by one dynamic program.
package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io/fs"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/vinodhalaharvi/collage/engine"
	"github.com/vinodhalaharvi/collage/render"
	"github.com/vinodhalaharvi/collage/web"
)

var version = "dev"

const maxPhotos = 10

const usage = `collage arranges photos into a collage of any shape.

Usage:
  collage [flags] photo...      make a collage image
  collage serve [-port N]       open the interactive editor in your browser
  collage laws                  verify the semiring laws and the DP
  collage version

Flags for making a collage:
  -o FILE         output file, .jpg or .png (default collage.jpg)
  -ratio R        canvas shape: 16:9, 1.5, or square, portrait, story,
                  landscape, banner (default square)
  -size PX        length of the longer canvas side (default 2400)
  -gutter PX      space between photos (default 12)
  -bg COLOR       background, #rrggbb (default #ffffff)
  -suggest N      write N structurally different layouts (FILE-1, FILE-2, …)

Up to 10 photos. JPEG, PNG and GIF everywhere; HEIC and WebP on macOS.

Examples:
  collage ~/Pictures/trip/*.jpg
  collage -ratio story -o story.jpg a.jpg b.jpg c.jpg d.jpg
  collage -ratio 3:1 -gutter 0 -suggest 3 *.png
`

func main() {
	// The solver makes many short-lived allocations; collecting less often is ~20% faster.
	debug.SetGCPercent(400)
	args := os.Args[1:]
	var err error
	switch {
	case len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help":
		fmt.Print(usage)
		return
	case args[0] == "version" || args[0] == "--version":
		fmt.Println("collage", version)
		return
	case args[0] == "serve":
		err = serve(args[1:])
	case args[0] == "laws":
		err = laws()
	case args[0] == "make":
		err = makeCollage(args[1:])
	default:
		err = makeCollage(args)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "collage:", err)
		os.Exit(1)
	}
}

// parseInterleaved lets flags appear before, between or after file names.
func parseInterleaved(fsf *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fsf.Parse(args); err != nil {
			return nil, err
		}
		if fsf.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fsf.Arg(0))
		args = fsf.Args()[1:]
	}
}

func parseRatio(s string) (float64, error) {
	named := map[string]float64{"square": 1, "portrait": 4.0 / 5, "story": 9.0 / 16, "landscape": 16.0 / 9, "banner": 3}
	if r, ok := named[strings.ToLower(s)]; ok {
		return r, nil
	}
	if a, b, ok := strings.Cut(s, ":"); ok {
		w, err1 := strconv.ParseFloat(a, 64)
		h, err2 := strconv.ParseFloat(b, 64)
		if err1 == nil && err2 == nil && w > 0 && h > 0 {
			return w / h, nil
		}
	} else if r, err := strconv.ParseFloat(s, 64); err == nil && r > 0 {
		return r, nil
	}
	return 0, fmt.Errorf("can't read ratio %q: use 16:9, 1.5 or a name like story", s)
}

func parseColor(s string) (color.RGBA, error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if len(s) != 6 || err != nil {
		return color.RGBA{}, fmt.Errorf("can't read colour %q: use #rrggbb", s)
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, nil
}

func makeCollage(args []string) error {
	fsf := flag.NewFlagSet("collage", flag.ContinueOnError)
	fsf.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	out := fsf.String("o", "collage.jpg", "")
	ratioS := fsf.String("ratio", "square", "")
	size := fsf.Int("size", 2400, "")
	gutter := fsf.Int("gutter", 12, "")
	bgS := fsf.String("bg", "#ffffff", "")
	suggest := fsf.Int("suggest", 1, "")
	files, err := parseInterleaved(fsf, args)
	if err != nil {
		return err
	}
	files = skipOutputs(files, *out, *suggest)
	if len(files) == 0 {
		return errors.New("give at least one photo, e.g. collage *.jpg")
	}
	if len(files) > maxPhotos {
		return fmt.Errorf("up to %d photos per collage; you gave %d", maxPhotos, len(files))
	}
	ratio, err := parseRatio(*ratioS)
	if err != nil {
		return err
	}
	bg, err := parseColor(*bgS)
	if err != nil {
		return err
	}
	if *size < 64 || *size > 20000 {
		return errors.New("-size must be between 64 and 20000")
	}
	ext := strings.ToLower(filepath.Ext(*out))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return fmt.Errorf("output must end in .jpg or .png, got %q", *out)
	}

	photos := make([]render.Photo, len(files))
	aspects := make([]float64, len(files))
	for i, f := range files {
		p, err := render.Probe(f)
		if err != nil {
			return err
		}
		defer p.Cleanup()
		photos[i] = p
		aspects[i] = math.Min(engine.MaxAspect/2, math.Max(2/engine.MaxAspect, p.Aspect()))
	}

	t0 := time.Now()
	sol := engine.Solve(aspects)
	elapsed := time.Since(t0)
	_, total := sol.Histogram()
	layouts := sol.Layouts(ratio, max(1, *suggest))
	if len(layouts) == 0 {
		return errors.New("no layout fits these photos")
	}

	W, H := *size, int(math.Round(float64(*size)/ratio))
	if ratio < 1 {
		W, H = int(math.Round(float64(*size)*ratio)), *size
	}
	for i, l := range layouts {
		name := *out
		if *suggest > 1 {
			name = strings.TrimSuffix(*out, filepath.Ext(*out)) + "-" + strconv.Itoa(i+1) + filepath.Ext(*out)
		}
		img, err := render.Render(l.Tree, photos, render.Options{Width: W, Height: H, Gutter: *gutter, Background: bg})
		if err != nil {
			return err
		}
		if err := save(name, img); err != nil {
			return err
		}
		fmt.Printf("%s  %d×%d  smallest photo %.1f%% · cropped %.1f%%\n", name, W, H, 100*l.MinShare, 100*l.Crop)
	}
	fmt.Printf("searched %s layouts of %d photos in %s\n", commas(total), len(files), elapsed.Round(time.Millisecond))
	return nil
}

// skipOutputs drops files this run will overwrite, so `collage *.jpg` run twice
// does not put the previous collage inside the new one.
func skipOutputs(files []string, out string, suggest int) []string {
	outs := map[string]bool{}
	add := func(p string) {
		if a, err := filepath.Abs(p); err == nil {
			outs[a] = true
		}
	}
	add(out)
	for i := 1; i <= max(1, suggest); i++ {
		add(strings.TrimSuffix(out, filepath.Ext(out)) + "-" + strconv.Itoa(i) + filepath.Ext(out))
	}
	var keep []string
	for _, f := range files {
		if a, err := filepath.Abs(f); err == nil && outs[a] {
			fmt.Fprintf(os.Stderr, "skipping %s (it is the output file)\n", f)
			continue
		}
		keep = append(keep, f)
	}
	return keep
}

func save(name string, img image.Image) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	if strings.HasSuffix(strings.ToLower(name), ".png") {
		err = png.Encode(f, img)
	} else {
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 92})
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

func commas(x float64) string {
	s := strconv.FormatFloat(math.Round(x), 'f', 0, 64)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func serve(args []string) error {
	fsf := flag.NewFlagSet("serve", flag.ContinueOnError)
	port := fsf.Int("port", 7777, "port to listen on")
	noOpen := fsf.Bool("no-open", false, "don't open a browser")
	if err := fsf.Parse(args); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}
	}
	url := "http://" + ln.Addr().String() + "/"
	fmt.Println("collage editor running at", url, "(Ctrl-C to stop)")
	if !*noOpen {
		openBrowser(url)
	}
	sub, _ := fs.Sub(web.FS, ".")
	return http.Serve(ln, http.FileServer(http.FS(sub)))
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func laws() error {
	fmt.Println("Semiring laws (10,000 random trials each)")
	fmt.Println()
	fail := false
	cur := ""
	for _, r := range engine.AllLaws(10000, time.Now().UnixNano()) {
		if r.Algebra != cur {
			cur = r.Algebra
			fmt.Println(" ", cur)
		}
		mark := "holds"
		if !r.OK {
			mark, fail = "FAILS", true
		}
		fmt.Printf("    %-22s %s\n", r.Law, mark)
	}
	fmt.Println()
	fmt.Println("Dynamic program vs brute force (every layout folded one by one)")
	for n := 2; n <= 6; n++ {
		c := engine.VerifyDyna(n, time.Now().UnixNano())
		mark := "equal"
		if !c.OK {
			mark, fail = "DIFFERENT", true
		}
		fmt.Printf("    %d photos  %7s layouts  %4d steps over %2d subproblems  %s\n", n, commas(float64(c.Trees)), c.Alternatives, c.States, mark)
	}
	if fail {
		return errors.New("a law failed")
	}
	return nil
}
