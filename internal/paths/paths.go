package paths

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// SupportedInputExtensions lists every extension the engine layer can read.
// .apng is here because an animated PNG is a PNG with an animation chunk, and
// the convention of naming it .apng is what lets the GIF tool find it.
var SupportedInputExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".apng": true, ".webp": true, ".gif": true,
	".bmp": true, ".tif": true, ".tiff": true, ".ico": true, ".svg": true,
	".avif": true, ".heic": true, ".heif": true,
}

// ImageExtensions is the dialog filter set, without SVG.
var ImageExtensions = []string{
	"jpg", "jpeg", "png", "apng", "webp", "gif", "bmp", "tif", "tiff", "ico", "avif", "heic", "heif", "svg",
}

// ErrEmptyPath reports an empty or whitespace-only user path.
var ErrEmptyPath = errors.New("路径不能为空")

// Normalize validates a user-supplied path and returns it cleaned.
func Normalize(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ErrEmptyPath
	}
	if strings.ContainsRune(trimmed, 0) {
		return "", fmt.Errorf("路径包含空字符")
	}
	// Reject relative parent traversal that could escape the working directory.
	if strings.HasPrefix(trimmed, "..") && !filepath.IsAbs(trimmed) {
		parent := trimmed
		parent = strings.TrimLeft(parent, ".")
		if strings.HasPrefix(parent, "/") || strings.HasPrefix(parent, `\`) {
			return "", fmt.Errorf("路径不允许以 .. 开头")
		}
	}
	return filepath.Clean(trimmed), nil
}

// NormalizeOptional normalizes a path that may legitimately be empty.
func NormalizeOptional(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	out, err := Normalize(value)
	if err != nil {
		return strings.TrimSpace(value)
	}
	return out
}

// InputFile describes one expanded input path.
type InputFile struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	ModTimeNS int64  `json:"mod_time_ns"`
	Size      int64  `json:"size"`
	// RelPath is the file's path relative to the folder it was found in, and is
	// empty for a file the user dropped directly. It is what
	// preserve_folder_structure reproduces under the output directory.
	RelPath string `json:"rel_path,omitempty"`
}

// ExpandResult is what a drag-and-drop expansion returns.
type ExpandResult struct {
	Files     []InputFile `json:"files"`
	InputDirs []string    `json:"input_dirs"`
	Skipped   []string    `json:"skipped"`
}

// Expand turns dropped paths into a flat, de-duplicated list of image files.
// Directories are walked recursively; unsupported extensions are reported in
// Skipped instead of being silently dropped.
func Expand(values []string) ExpandResult {
	return ExpandContext(context.Background(), values)
}

// ExpandContext 在目录遍历与文件探测之间响应取消。
func ExpandContext(ctx context.Context, values []string) ExpandResult {
	out := ExpandResult{Files: []InputFile{}, InputDirs: []string{}, Skipped: []string{}}
	seen := map[string]bool{}
	dirs := map[string]bool{}

	add := func(path, rel string) {
		if ctx.Err() != nil {
			return
		}
		key := identityKey(path)
		if seen[key] {
			return
		}
		info, err := os.Stat(path)
		if err != nil {
			out.Skipped = append(out.Skipped, path)
			return
		}
		if !SupportedInputExtensions[strings.ToLower(filepath.Ext(path))] {
			out.Skipped = append(out.Skipped, path)
			return
		}
		seen[key] = true
		mod := info.ModTime().UnixNano()
		out.Files = append(out.Files, InputFile{
			Path:      path,
			Name:      filepath.Base(path),
			ModTimeNS: mod,
			Size:      info.Size(),
			RelPath:   rel,
		})
	}

	for _, raw := range values {
		if ctx.Err() != nil {
			break
		}
		path := NormalizeOptional(raw)
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			out.Skipped = append(out.Skipped, path)
			continue
		}
		if !info.IsDir() {
			add(path, "")
			continue
		}
		key := identityKey(path)
		if !dirs[key] {
			dirs[key] = true
			out.InputDirs = append(out.InputDirs, path)
		}
		var walked []string
		_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			walked = append(walked, p)
			return nil
		})
		sort.Strings(walked)
		for _, p := range walked {
			rel, rerr := filepath.Rel(path, p)
			if rerr != nil {
				rel = ""
			}
			add(p, rel)
		}
	}
	sort.SliceStable(out.Files, func(i, j int) bool { return out.Files[i].Path < out.Files[j].Path })
	return out
}

// SafeName 将模板结果限制为单个跨平台文件名。
func SafeName(value string) string {
	value = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, value)
	value = strings.Trim(value, " .")
	if value == "" {
		value = "image"
	}
	runes := []rune(value)
	if len(runes) > 100 {
		value = string(runes[:100])
	}
	stem := strings.ToUpper(strings.SplitN(value, ".", 2)[0])
	stemRunes := []rune(stem)
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || (len(stemRunes) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && strings.ContainsRune("123456789¹²³", stemRunes[3])) {
		value = "_" + value
	}
	return value
}

// identityKey makes two spellings of one path compare equal on Windows.
func identityKey(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	if runtime.GOOS == "windows" {
		abs = strings.ToLower(abs)
	}
	return abs
}

// SamePath reports whether two paths name the same file.
func SamePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return identityKey(a) == identityKey(b)
}

// Exists reports whether path names an existing file or directory.
func Exists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// Naming produces an output file name from the user's template.
type Naming struct {
	Prefix   string
	Template string
}

// Render resolves the placeholder template for one input file.
//
// Supported placeholders: {prefix} {basename} {op} {date:YYYYMMDD}
// {time:HHmmss} {seq:N}
func (n Naming) Render(inputPath, op string, seq, year, month, day, hour, minute, second int) string {
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	replacer := strings.NewReplacer(
		"{prefix}", n.Prefix,
		"{basename}", base,
		"{op}", op,
	)
	out := replacer.Replace(n.Template)
	out = replaceDate(out, "{date:", fmt.Sprintf("%04d%02d%02d", year, month, day))
	out = replaceDate(out, "{time:", fmt.Sprintf("%02d%02d%02d", hour, minute, second))
	out = replaceSeq(out, seq)
	if strings.TrimSpace(out) == "" {
		out = base
	}
	return SafeName(out)
}

func replaceDate(s, marker, value string) string {
	for {
		start := strings.Index(s, marker)
		if start < 0 {
			return s
		}
		end := strings.Index(s[start:], "}")
		if end < 0 {
			return s
		}
		s = s[:start] + value + s[start+end+1:]
	}
}

func replaceSeq(s string, seq int) string {
	const marker = "{seq"
	for {
		start := strings.Index(s, marker)
		if start < 0 {
			return s
		}
		end := strings.Index(s[start:], "}")
		if end < 0 {
			return s
		}
		inner := s[start+len(marker) : start+end]
		width := 1
		if strings.HasPrefix(inner, ":") {
			if n, err := strconv.Atoi(inner[1:]); err == nil && n > 0 && n <= 6 {
				width = n
			}
		}
		s = s[:start] + fmt.Sprintf("%0*d", width, seq) + s[start+end+1:]
	}
}

// ResolveOutput picks a non-conflicting output path under strategy.
//
// reserved holds paths already planned in the same batch.
func ResolveOutput(base string, reserved []string, strategy string) (string, bool, error) {
	return resolveOutput(base, strategy, func(path string) bool { return containsPath(reserved, path) })
}

type OutputReservations map[string]struct{}

func (reserved *OutputReservations) Reserve(base, strategy string) (string, bool, error) {
	resolved, exists, err := resolveOutput(base, strategy, func(path string) bool {
		_, taken := (*reserved)[identityKey(path)]
		return taken
	})
	if err != nil || strategy == "skip" && exists {
		return resolved, exists, err
	}
	if *reserved == nil {
		*reserved = make(OutputReservations)
	}
	(*reserved)[identityKey(resolved)] = struct{}{}
	return resolved, exists, nil
}

func resolveOutput(base, strategy string, reserved func(string) bool) (string, bool, error) {
	base, err := Normalize(base)
	if err != nil {
		return "", false, err
	}
	exists := Exists(base)
	dup := reserved(base)
	switch strategy {
	case "overwrite":
		if !dup {
			return base, exists, nil
		}
	case "skip":
		return base, exists || dup, nil
	}
	if !exists && !dup {
		return base, false, nil
	}
	dir := filepath.Dir(base)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(filepath.Base(base), ext)
	for i := 1; i <= 9999; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", stem, i, ext))
		if !reserved(candidate) && !Exists(candidate) {
			return candidate, false, nil
		}
	}
	return "", false, fmt.Errorf("无法为 %s 找到可用的输出路径", base)
}

func containsPath(list []string, path string) bool {
	key := identityKey(path)
	for _, item := range list {
		if identityKey(item) == key {
			return true
		}
	}
	return false
}
