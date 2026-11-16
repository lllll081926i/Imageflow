package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// ConflictStrategy mirrors ImageFlow's output conflict policies.
type ConflictStrategy string

const (
	ConflictRename    ConflictStrategy = "rename"
	ConflictOverwrite ConflictStrategy = "overwrite"
	ConflictSkip      ConflictStrategy = "skip"
)

// Settings is the persisted application configuration.
type Settings struct {
	MaxConcurrency          int              `json:"max_concurrency"`
	OutputPrefix            string           `json:"output_prefix"`
	OutputTemplate          string           `json:"output_template"`
	PreserveFolderStructure bool             `json:"preserve_folder_structure"`
	ConflictStrategy        ConflictStrategy `json:"conflict_strategy"`
	DefaultOutputDir        string           `json:"default_output_dir"`
	RecentInputDirs         []string         `json:"recent_input_dirs"`
	RecentOutputDirs        []string         `json:"recent_output_dirs"`
	AutoCheckUpdates        bool             `json:"auto_check_updates"`
	OpenOutputOnComplete    bool             `json:"open_output_folder_on_complete"`
	RememberWindowState     bool             `json:"remember_window_state"`
	UITransparency          bool             `json:"ui_transparency"`
	UIReducedMotion         bool             `json:"ui_reduced_motion"`
	BackgroundRunning       bool             `json:"background_running"`
	// RailOrder is the sidebar's row order: position → index into Tools. It is a
	// permutation, so a tool keeps its identity, its page and its selection while
	// only its position changes. A missing or stale list falls back to the natural
	// order.
	RailOrder []int `json:"rail_order,omitempty"`
	// Collage keeps the collage page's parameters across sessions: a template, a
	// canvas ratio and a set of annotations take minutes to put together, and
	// losing them on every launch made the page expensive to use.
	Collage Collage `json:"collage"`
}

// Collage is the persisted state of the collage page.
type Collage struct {
	GroupCount int    `json:"group_count"`
	PresetID   string `json:"preset_id"`
	UseCustom  bool   `json:"use_custom"`
	GridCols   int    `json:"grid_cols"`
	GridRows   int    `json:"grid_rows"`
	// GridParents is the merge editor's cell→block map, row-major.
	GridParents []int `json:"grid_parents,omitempty"`

	AspectID     string `json:"aspect_id"`
	CustomRatio  string `json:"custom_ratio"`
	LongEdge     int    `json:"long_edge"`
	MarginTop    int    `json:"margin_top"`
	MarginRight  int    `json:"margin_right"`
	MarginBottom int    `json:"margin_bottom"`
	MarginLeft   int    `json:"margin_left"`
	// Seam is 0 for a seamless collage and 1 for one with a gap.
	Seam   int `json:"seam"`
	Gap    int `json:"gap"`
	Radius int `json:"radius"`
	Fit    int `json:"fit"`

	Background    string `json:"background"`
	BackgroundDim int    `json:"background_dim"`

	Mode    int `json:"mode"`
	Format  int `json:"format"`
	Quality int `json:"quality"`

	Watermark        bool    `json:"watermark"`
	WatermarkText    string  `json:"watermark_text"`
	WatermarkSize    int     `json:"watermark_size"`
	WatermarkOpacity float64 `json:"watermark_opacity"`
	WatermarkAngle   float64 `json:"watermark_angle"`
	WatermarkDensity int     `json:"watermark_density"`

	// Marks are the canvas annotations. They hold no file paths, so they stay
	// meaningful when the pictures change.
	Marks []CollageMark `json:"marks,omitempty"`
}

// CollageMark is one persisted annotation, in canvas fractions.
type CollageMark struct {
	Kind  string  `json:"kind"`
	Text  string  `json:"text,omitempty"`
	X1    float64 `json:"x1"`
	Y1    float64 `json:"y1"`
	X2    float64 `json:"x2"`
	Y2    float64 `json:"y2"`
	Color string  `json:"color,omitempty"`
	Width int     `json:"width,omitempty"`
}

// DefaultCollage is the collage page's shipping state: a four-picture template
// on a 1:1 seamless canvas.
func DefaultCollage() Collage {
	return Collage{
		GroupCount:       4,
		GridCols:         3,
		GridRows:         3,
		AspectID:         "1:1",
		CustomRatio:      "16:9",
		LongEdge:         1600,
		Seam:             0,
		Gap:              10,
		Radius:           8,
		Background:       "#FFFFFF",
		BackgroundDim:    35,
		Mode:             0,
		Format:           0,
		Quality:          92,
		WatermarkText:    "ImageFlow",
		WatermarkSize:    28,
		WatermarkOpacity: 0.25,
		WatermarkAngle:   -30,
	}
}

// DefaultConcurrency picks a conservative worker count per platform.
func DefaultConcurrency() int {
	n := runtime.NumCPU()
	if runtime.GOOS == "windows" {
		if n < 2 {
			return 1
		}
		if n/2 > 4 {
			return 4
		}
		return n / 2
	}
	if n > 8 {
		return 8
	}
	if n < 1 {
		return 1
	}
	return n
}

// Default returns the settings a fresh install uses.
func Default() Settings {
	return Settings{
		MaxConcurrency:          DefaultConcurrency(),
		OutputPrefix:            "IF",
		OutputTemplate:          "{prefix}{basename}",
		PreserveFolderStructure: true,
		ConflictStrategy:        ConflictRename,
		AutoCheckUpdates:        true,
		RememberWindowState:     true,
		UITransparency:          true,
		Collage:                 DefaultCollage(),
	}
}

// Path resolves the settings file location for the current platform.
func Path() (string, error) {
	if override := os.Getenv("IMAGEFLOW_SETTINGS_FILE"); override != "" {
		return override, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "imageflow", "settings.json"), nil
}

// Load reads the settings file, falling back to defaults when it is missing
// or unreadable. A malformed file is not fatal: the app starts with defaults.
func Load() Settings {
	out := Default()
	path, err := Path()
	if err != nil {
		return out
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Default()
	}
	out.normalize()
	return out
}

func (s *Settings) normalize() {
	if s.MaxConcurrency < 1 || s.MaxConcurrency > 32 {
		s.MaxConcurrency = DefaultConcurrency()
	}
	switch s.ConflictStrategy {
	case ConflictRename, ConflictOverwrite, ConflictSkip:
	default:
		s.ConflictStrategy = ConflictRename
	}
	if s.OutputTemplate == "" {
		s.OutputTemplate = "{prefix}{basename}"
	}
	if s.OutputPrefix == "" {
		s.OutputPrefix = "IF"
	}
	if len(s.RecentInputDirs) > 4 {
		s.RecentInputDirs = s.RecentInputDirs[:4]
	}
	if len(s.RecentOutputDirs) > 4 {
		s.RecentOutputDirs = s.RecentOutputDirs[:4]
	}
	s.Collage.normalize()
}

// normalize repairs a collage block read from an older or hand-edited file, so a
// missing or impossible value falls back to the shipping default instead of
// rendering an empty canvas.
func (c *Collage) normalize() {
	d := DefaultCollage()
	if c.GroupCount < 1 || c.GroupCount > 16 {
		c.GroupCount = d.GroupCount
	}
	if c.GridCols < 1 || c.GridCols > 8 {
		c.GridCols = d.GridCols
	}
	if c.GridRows < 1 || c.GridRows > 8 {
		c.GridRows = d.GridRows
	}
	if c.GridParents != nil && len(c.GridParents) != c.GridCols*c.GridRows {
		// A merge map that does not match the grid would index out of range.
		c.GridParents = nil
	}
	if c.AspectID == "" {
		c.AspectID = d.AspectID
	}
	if c.CustomRatio == "" {
		c.CustomRatio = d.CustomRatio
	}
	if c.LongEdge < 64 || c.LongEdge > 16000 {
		c.LongEdge = d.LongEdge
	}
	if c.Seam != 0 && c.Seam != 1 {
		c.Seam = d.Seam
	}
	if c.Gap < 0 || c.Gap > 400 {
		c.Gap = d.Gap
	}
	if c.Radius < 0 || c.Radius > 400 {
		c.Radius = d.Radius
	}
	if c.Fit < 0 || c.Fit > 2 {
		c.Fit = 0
	}
	if c.Background == "" {
		c.Background = d.Background
	}
	if c.BackgroundDim < 0 || c.BackgroundDim > 100 {
		c.BackgroundDim = d.BackgroundDim
	}
	if c.Mode != 0 && c.Mode != 1 {
		c.Mode = d.Mode
	}
	if c.Format < 0 || c.Format > 2 {
		c.Format = d.Format
	}
	if c.Quality < 1 || c.Quality > 100 {
		c.Quality = d.Quality
	}
	if c.WatermarkText == "" {
		c.WatermarkText = d.WatermarkText
	}
	if c.WatermarkSize < 10 || c.WatermarkSize > 120 {
		c.WatermarkSize = d.WatermarkSize
	}
	if c.WatermarkOpacity <= 0 || c.WatermarkOpacity > 1 {
		c.WatermarkOpacity = d.WatermarkOpacity
	}
	if c.WatermarkDensity < 0 || c.WatermarkDensity > 2 {
		c.WatermarkDensity = 0
	}
	marks := c.Marks[:0]
	for _, m := range c.Marks {
		switch m.Kind {
		case "text", "arrow", "rect", "ellipse":
			marks = append(marks, m)
		}
	}
	c.Marks = marks
}

// Save writes the settings atomically, through a temporary file in the same
// directory, so an interrupted write never leaves a truncated settings file. The
// flush is explicit: a save is rare, and a power loss right after it must not
// leave a zero-length file.
func Save(s Settings) error {
	path, err := Path()
	if err != nil {
		return err
	}
	s.normalize()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// PushRecent returns the list with value moved to the front, kept unique on a
// case-insensitive comparison, and capped at four entries.
func PushRecent(current []string, value string) []string {
	value = normalizeRecent(value)
	if value == "" {
		return current
	}
	out := []string{value}
	seen := map[string]bool{recentKey(value): true}
	for _, item := range current {
		candidate := normalizeRecent(item)
		if candidate == "" {
			continue
		}
		key := recentKey(candidate)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, candidate)
		if len(out) >= 4 {
			break
		}
	}
	return out
}

func normalizeRecent(v string) string {
	v = trimSpaces(v)
	if v == "" {
		return ""
	}
	if v == "/" || v == `\` {
		return v
	}
	return filepath.Clean(v)
}

func recentKey(v string) string {
	if runtime.GOOS == "windows" {
		return lowerASCII(v)
	}
	return v
}

func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func trimSpaces(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
