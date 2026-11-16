package ui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	myui "github.com/egoist/mygo/ui"

	"github.com/lllll081926i/imageflow-go/internal/batch"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("IMAGEFLOW_SETTINGS_FILE", filepath.Join(dir, "settings.json"))
	return NewApp(settings.Default())
}

// dumpFrame renders the current view to a PNG so a broken layout can be
// inspected instead of guessed at.
func dumpFrame(t *testing.T, a *App, name string) string {
	t.Helper()
	img := myui.Render(a.View, 1440, 900, 1)
	if img == nil {
		t.Fatal("render returned no image")
	}
	if hashes(img) < 3 {
		t.Fatalf("%s: frame is blank (%d distinct colours)", name, hashes(img))
	}
	path := filepath.Join("testdata", name+".png")
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

// hashes counts distinct sampled colours, so a blank frame fails the check.
func hashes(img *image.RGBA) int {
	seen := map[uint32]bool{}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += 9 {
		for x := b.Min.X; x < b.Max.X; x += 9 {
			c := img.RGBAAt(x, y)
			seen[uint32(c.R)<<16|uint32(c.G)<<8|uint32(c.B)] = true
		}
	}
	return len(seen)
}

func TestEveryToolPageRenders(t *testing.T) {
	a := newTestApp(t)
	for i, tool := range a.Tools {
		a.Active = i
		a.ShowSettings = false
		tt := myui.NewTester(a.View, 1440, 900)
		if !tt.HasText(tool.Name()) {
			t.Fatalf("tool %d (%s): title missing, texts=%v", i, tool.Name(), tt.Texts())
		}
		path := dumpFrame(t, a, fmt.Sprintf("tool-%02d", i))
		if st, err := os.Stat(path); err != nil || st.Size() < 2000 {
			t.Fatalf("tool %d screenshot too small: %v", i, err)
		}
	}
}

func TestDarkModeRenders(t *testing.T) {
	a := newTestApp(t)
	a.Active = 0
	tt := myui.NewTester(a.View, 1440, 900)
	tt.SetDark(true)
	tt.Frame()
	img := tt.Image()
	if img == nil || hashes(img) < 3 {
		t.Fatal("dark frame is blank")
	}
	// A dark frame must actually be darker than the light one.
	if luminance(img) > 120 {
		t.Fatalf("dark frame luminance = %.0f, theme did not apply", luminance(img))
	}
	dumpImage(t, img, "dark-tool-00")

	a.ShowSettings = true
	// The page handover animates; advance past it so the screenshot shows the
	// settled page rather than the first frame of the lift.
	for i := 0; i < 40; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	dumpImage(t, tt.Image(), "dark-settings")
}

// luminance samples the frame's mean brightness.
func luminance(img *image.RGBA) float64 {
	b := img.Bounds()
	var sum float64
	var n int
	for y := b.Min.Y; y < b.Max.Y; y += 11 {
		for x := b.Min.X; x < b.Max.X; x += 11 {
			c := img.RGBAAt(x, y)
			sum += 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// dumpImage writes any rendered frame to testdata.
func dumpImage(t *testing.T, img *image.RGBA, name string) string {
	t.Helper()
	if img == nil {
		t.Fatal("no image to dump")
	}
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", name+".png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSettingsPageRenders(t *testing.T) {
	a := newTestApp(t)
	a.ShowSettings = true
	tt := myui.NewTester(a.View, 1440, 900)
	for _, want := range []string{"设置", "输出命名", "同名文件", "并发与行为"} {
		if !tt.HasText(want) {
			t.Errorf("%q missing from settings: %v", want, tt.Texts())
		}
	}
	dumpFrame(t, a, "settings")
}

func TestNavigationSwitchesTools(t *testing.T) {
	a := newTestApp(t)
	tt := myui.NewTester(a.View, 1440, 900)
	if err := tt.Click("批量水印"); err != nil {
		t.Fatalf("click: %v", err)
	}
	if got := a.ActiveTool().Name(); got != "批量水印" {
		t.Fatalf("active = %q", got)
	}
	if !tt.HasText("水印内容为空") && !tt.HasText("平铺重复") {
		t.Errorf("watermark controls missing: %v", tt.Texts())
	}
	if err := tt.Click("转 PDF"); err != nil {
		t.Fatalf("click pdf: %v", err)
	}
	if got := a.ActiveTool().Name(); got != "转 PDF" {
		t.Errorf("active = %q", got)
	}
}

func TestEmptyListShowsPlaceholder(t *testing.T) {
	a := newTestApp(t)
	tt := myui.NewTester(a.View, 1440, 900)
	if !tt.HasText("点击选择文件，或拖入文件夹") {
		t.Errorf("placeholder missing: %v", tt.Texts())
	}
	if !tt.HasText("输入项") {
		t.Errorf("count missing: %v", tt.Texts())
	}
}

func TestToolRunsWithoutItemsReportsStatus(t *testing.T) {
	a := newTestApp(t)
	a.Start()
	if a.StatusLine() != "先添加图片" {
		t.Fatalf("status = %q", a.StatusLine())
	}
	if a.IsRunning() {
		t.Error("started a run with no items")
	}
}

func TestSettingsSavePersists(t *testing.T) {
	a := newTestApp(t)
	a.ShowSettings = true
	a.Settings.OutputPrefix = "ZZ"
	tt := myui.NewTester(a.View, 1440, 900)
	if err := tt.Click("保存设置"); err != nil {
		t.Fatalf("click save: %v", err)
	}
	if a.StatusLine() != "设置已保存" {
		t.Fatalf("status = %q", a.StatusLine())
	}
	if got := settings.Load().OutputPrefix; got != "ZZ" {
		t.Errorf("persisted prefix = %q", got)
	}
}

func TestHelpers(t *testing.T) {
	if got := truncate("abcdefghij", 5); got != "abcd…" {
		t.Errorf("truncate = %q", got)
	}
	if got := truncate("abc", 5); got != "abc" {
		t.Errorf("truncate short = %q", got)
	}
	if got := indexOf([]string{"a", "b"}, "B"); got != 1 {
		t.Errorf("indexOf case-insensitive = %d", got)
	}
	if got := extOf("JPEG"); got != "jpg" {
		t.Errorf("extOf = %q", got)
	}
	if got := extOf("apng"); got != "apng" {
		t.Errorf("extOf apng = %q", got)
	}
	if got := hexOf(myui.RGB(255, 128, 0)); got != "#FF8000" {
		t.Errorf("hexOf = %q", got)
	}
	if got := colorOf("#00FF00"); got.R != 0 || got.G != 255 {
		t.Errorf("colorOf = %+v", got)
	}
	if got := baseNameOf("/tmp/dir/photo.jpeg"); got != "photo" {
		t.Errorf("baseNameOf = %q", got)
	}
	if got := outputDirFor(settings.Default(), batch.Item{Path: "/tmp/a.png"}, "_grid"); filepath.Base(got) != "a_grid" {
		t.Errorf("outputDirFor = %q", got)
	}
}

func TestOutputPathHonoursTemplate(t *testing.T) {
	a := newTestApp(t)
	s := a.Session.Settings
	s.OutputPrefix = "IF"
	s.OutputTemplate = "{prefix}_{basename}_{op}"
	s.DefaultOutputDir = filepath.Join("out", "dir")
	if err := a.Session.SetSettings(s); err != nil {
		t.Fatal(err)
	}
	a.resetPlan()
	got, skipped := a.outputPathFor(batch.Item{Path: "/tmp/photo.png"}, "convert", "png", 1)
	if skipped {
		t.Fatal("a fresh path must not be skipped")
	}
	want := filepath.Join("out", "dir", "IF_photo_convert.png")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestOutputPathAvoidsCollisionsInsideOneBatch(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	s := a.Session.Settings
	s.OutputTemplate = "{basename}"
	s.DefaultOutputDir = dir
	if err := a.Session.SetSettings(s); err != nil {
		t.Fatal(err)
	}
	a.resetPlan()
	first, skipped := a.outputPathFor(batch.Item{Path: filepath.Join(dir, "a", "same.png")}, "convert", "png", 1)
	if skipped {
		t.Fatal("first row was skipped")
	}
	second, skipped := a.outputPathFor(batch.Item{Path: filepath.Join(dir, "b", "same.png")}, "convert", "png", 2)
	if skipped {
		t.Fatal("second row was skipped")
	}
	if first == second {
		t.Fatalf("two rows resolved to the same path: %s", first)
	}
	if filepath.Base(second) != "same_1.png" {
		t.Errorf("second = %q", filepath.Base(second))
	}
}

func TestParseIcoSizes(t *testing.T) {
	got := parseIcoSizes("16,32,64")
	if len(got) != 3 || got[0] != 16 || got[2] != 64 {
		t.Fatalf("got %v", got)
	}
	if def := parseIcoSizes("bad"); len(def) != 4 {
		t.Errorf("fallback = %v", def)
	}
	if filtered := parseIcoSizes("4,16,9999"); len(filtered) != 1 || filtered[0] != 16 {
		t.Errorf("range filter = %v", filtered)
	}
}
