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
