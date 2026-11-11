package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

// FontChoice names one selectable font family.
type FontChoice struct {
	Label string
	File  string
}

// fontCandidates per platform, ordered by preference.
var fontCandidates = []FontChoice{
	{"微软雅黑", "msyh.ttc"},
	{"微软雅黑", "msyh.ttf"},
	{"黑体", "simhei.ttf"},
	{"宋体", "simsun.ttc"},
	{"等线", "Deng.ttf"},
	{"Arial", "arial.ttf"},
	{"苹方", "PingFang.ttc"},
	{"Noto Sans CJK", "NotoSansCJK-Regular.ttc"},
	{"Noto Sans CJK", "NotoSansCJK-Regular.ttf"},
	{"DejaVu Sans", "DejaVuSans.ttf"},
	{"Liberation Sans", "LiberationSans-Regular.ttf"},
}

var fontDirs = func() []string {
	switch runtime.GOOS {
	case "windows":
		windir := os.Getenv("WINDIR")
		if windir == "" {
			windir = `C:\Windows`
		}
		return []string{filepath.Join(windir, "Fonts")}
	case "darwin":
		return []string{
			"/System/Library/Fonts",
			"/System/Library/Fonts/Supplemental",
			"/Library/Fonts",
		}
	default:
		return []string{
			"/usr/share/fonts",
			"/usr/local/share/fonts",
			filepath.Join(os.Getenv("HOME"), ".local/share/fonts"),
			filepath.Join(os.Getenv("HOME"), ".fonts"),
		}
	}
}()

var (
	fontIndexOnce sync.Once
	fontIndex     map[string]string // lower-case base name -> absolute path

	faceCacheMu sync.Mutex
	faceCache   = map[faceKey]*fontHolder{}

	// fontMu serialises every glyph operation: x/image/font/opentype caches
	// glyph data inside sfnt.Font without a lock, so concurrent shapes on one
	// face corrupt that cache.
	fontMu sync.Mutex
)

type faceKey struct {
	path string
	size int
	dpi  int
}

type fontHolder struct {
	face font.Face
	sfnt *sfnt.Font
	size int
}

func buildFontIndex() {
	fontIndex = map[string]string{}
	for _, dir := range fontDirs {
		_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext != ".ttf" && ext != ".ttc" && ext != ".otf" {
				return nil
			}
			key := strings.ToLower(filepath.Base(p))
			if _, exists := fontIndex[key]; !exists {
				fontIndex[key] = p
			}
			return nil
		})
	}
}

// AvailableFonts lists the families this machine actually has.
func AvailableFonts() []string {
	fontIndexOnce.Do(buildFontIndex)
	seen := map[string]bool{}
	var out []string
	for _, c := range fontCandidates {
		path, ok := fontIndex[strings.ToLower(c.File)]
		if !ok || seen[c.Label] {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			continue
		}
		seen[c.Label] = true
		out = append(out, c.Label)
	}
	if len(out) == 0 {
		out = append(out, "系统默认")
	}
	sort.Strings(out)
	return out
}

// PreferredFont is the family a fresh panel should start on: the first entry of
// the platform's preference list that is actually installed. AvailableFonts sorts
// alphabetically for the picker, so taking its first item would land on Arial
// rather than the CJK face the original app used.
func PreferredFont() string {
	fontIndexOnce.Do(buildFontIndex)
	for _, c := range fontCandidates {
		if _, ok := fontIndex[strings.ToLower(c.File)]; ok {
			return c.Label
		}
	}
	return ""
}

// resolveFontPath finds the file for a family label, falling back to any
// installed CJK-capable font.
func resolveFontPath(label string, bold bool) (string, error) {
	fontIndexOnce.Do(buildFontIndex)
	want := strings.TrimSpace(label)
	for _, c := range fontCandidates {
		if !strings.EqualFold(c.Label, want) {
			continue
		}
		if bold {
			stem := strings.TrimSuffix(c.File, filepath.Ext(c.File))
			if p, ok := fontIndex[strings.ToLower(stem+"bd"+filepath.Ext(c.File))]; ok {
				return p, nil
			}
			if p, ok := fontIndex[strings.ToLower(stem+"-Bold"+filepath.Ext(c.File))]; ok {
				return p, nil
			}
		}
		if p, ok := fontIndex[strings.ToLower(c.File)]; ok {
			return p, nil
		}
	}
	for _, c := range fontCandidates {
		if p, ok := fontIndex[strings.ToLower(c.File)]; ok {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到可用字体")
}

// loadSFNT parses a font file, taking face 0 from a collection.
func loadSFNT(path string) (*sfnt.Font, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if coll, err := opentype.ParseCollection(data); err == nil && coll.NumFonts() > 0 {
		f, ferr := coll.Font(0)
		if ferr != nil {
			return nil, ferr
		}
		return f, nil
	}
	return opentype.Parse(data)
}

// FontFace returns a cached face for the family at the given pixel size.
func FontFace(label string, size int, dpi int, bold bool) (font.Face, error) {
	if size < 1 {
		size = 16
	}
	if dpi < 1 {
		dpi = 72
	}
	path, err := resolveFontPath(label, bold)
	if err != nil {
		return nil, err
	}
	key := faceKey{path: path, size: size, dpi: dpi}
	faceCacheMu.Lock()
	defer faceCacheMu.Unlock()
	if h, ok := faceCache[key]; ok {
		return h.face, nil
	}
	f, err := loadSFNT(path)
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    float64(size),
		DPI:     float64(dpi),
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	faceCache[key] = &fontHolder{face: face, sfnt: f, size: size}
	return face, nil
}
