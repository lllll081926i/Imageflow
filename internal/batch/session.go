package batch

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lllll081926i/imageflow-go/internal/appcache"
	"github.com/lllll081926i/imageflow-go/internal/engine"
	"github.com/lllll081926i/imageflow-go/internal/paths"
	"github.com/lllll081926i/imageflow-go/internal/settings"
)

// Item is one input file in the current session.
type Item struct {
	id        uint64
	runID     uint64
	Path      string  `json:"path"`
	Name      string  `json:"name"`
	Size      int64   `json:"size"`
	ModTimeNS int64   `json:"mod_time_ns"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Format    string  `json:"format"`
	Thumb     string  `json:"thumb,omitempty"` // data URL of the cached thumbnail
	Status    string  `json:"status"`          // "", "ok", "failed", "skipped", "cancelled"
	Message   string  `json:"message,omitempty"`
	Output    string  `json:"output,omitempty"`
	Progress  float64 `json:"progress,omitempty"`
	// RelPath is the input's path inside the folder it came from, empty when the
	// file was dropped on its own. OutputDirFor uses it to rebuild the folder
	// tree under the output directory.
	RelPath string `json:"rel_path,omitempty"`
}

// Session is the mutable state shared by the UI and the batch runners.
type Session struct {
	Mu              sync.Mutex
	settingsMu      sync.Mutex
	settingsPath    string
	settingsPathErr error
	Items           []*Item
	Settings        settings.Settings
	OutputOp        string
	Cancel          *Cancel
	Progress        Progress
	Running         bool
	LastRun         time.Time
	nextItemID      uint64
	runID           uint64
	itemsByID       map[uint64]*Item
}

// NewSession creates an empty session with the given settings.
func NewSession(s settings.Settings) *Session {
	path, err := settings.Path()
	return &Session{Settings: cloneSettings(s), settingsPath: path, settingsPathErr: err}
}

// Add merges expanded files into the session, keeping existing rows.
func (s *Session) Add(files []paths.InputFile) int {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	existing := map[string]int{}
	for i, it := range s.Items {
		existing[it.Path] = i
	}
	added := 0
	for _, f := range files {
		if i, ok := existing[f.Path]; ok {
			s.Items[i].Size = f.Size
			s.Items[i].ModTimeNS = f.ModTimeNS
			continue
		}
		s.nextItemID++
		item := &Item{
			id:        s.nextItemID,
			Path:      f.Path,
			Name:      f.Name,
			Size:      f.Size,
			ModTimeNS: f.ModTimeNS,
			RelPath:   f.RelPath,
		}
		if s.itemsByID == nil {
			s.itemsByID = make(map[uint64]*Item)
		}
		s.itemsByID[item.id] = item
		s.Items = append(s.Items, item)
		existing[f.Path] = len(s.Items) - 1
		added++
	}
	return added
}

// Remove drops the row at index.
func (s *Session) Remove(index int) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if index < 0 || index >= len(s.Items) {
		return
	}
	s.removeAt(index)
}

func (s *Session) RemovePath(path string) int {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	for index, item := range s.Items {
		if item.Path == path {
			s.removeAt(index)
			return index
		}
	}
	return -1
}

func (s *Session) removeAt(index int) {
	delete(s.itemsByID, s.Items[index].id)
	copy(s.Items[index:], s.Items[index+1:])
	s.Items[len(s.Items)-1] = nil
	s.Items = s.Items[:len(s.Items)-1]
}

func (s *Session) ReorderFiles(order []string) bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.Running || len(order) != len(s.Items) {
		return false
	}
	byPath := make(map[string]*Item, len(s.Items))
	for _, item := range s.Items {
		byPath[item.Path] = item
	}
	reordered := make([]*Item, len(order))
	for index, path := range order {
		item, exists := byPath[path]
		if !exists {
			return false
		}
		reordered[index] = item
		delete(byPath, path)
	}
	s.Items = reordered
	return true
}

// Clear empties the file list.
func (s *Session) Clear() {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.Items = nil
	s.itemsByID = nil
	s.Progress = Progress{}
}

// Snapshot copies the rows for a stable render pass.
func (s *Session) Snapshot() []Item {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	out := make([]Item, len(s.Items))
	for i, it := range s.Items {
		out[i] = *it
	}
	return out
}

// Len returns the row count.
func (s *Session) Len() int {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return len(s.Items)
}

// ResetStatus clears per-row results before a new run.
func (s *Session) ResetStatus() {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.runID++
	if s.itemsByID == nil {
		s.itemsByID = make(map[uint64]*Item, len(s.Items))
	}
	for _, it := range s.Items {
		if it.id == 0 {
			s.nextItemID++
			it.id = s.nextItemID
		}
		it.runID = s.runID
		s.itemsByID[it.id] = it
		it.Status = ""
		it.Message = ""
		it.Output = ""
		it.Progress = 0
	}
	s.Progress = Progress{Total: len(s.Items)}
}

// SetRowStatus records one row's outcome.
func (s *Session) SetRowStatus(index int, status, message, output string) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if index < 0 || index >= len(s.Items) {
		return
	}
	s.Items[index].Status = status
	s.Items[index].Message = message
	s.Items[index].Output = output
}

func (s *Session) SetItemStatus(item Item, status, message, output string) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if current := s.itemsByID[item.id]; current != nil && current.runID == item.runID {
		current.Status = status
		current.Message = message
		current.Output = output
	}
}

// SetRowProgress records partial progress for one row.
func (s *Session) SetRowProgress(index int, value float64) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if index < 0 || index >= len(s.Items) {
		return
	}
	s.Items[index].Progress = value
}

// SetMeta fills the probed dimensions, format and thumbnail of a row.
func (s *Session) SetMeta(index int, width, height int, format, thumb string) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if index < 0 || index >= len(s.Items) {
		return
	}
	it := s.Items[index]
	it.Width = width
	it.Height = height
	it.Format = format
	if thumb != "" {
		it.Thumb = thumb
	}
}

// SetSettings replaces the settings and persists them.
func (s *Session) SetSettings(next settings.Settings) error {
	return s.UpdateSettings(func(current *settings.Settings) { *current = next })
}

// UpdateSettings 在同一事务中修改最新设置并落盘，失败时保持旧值。
func (s *Session) UpdateSettings(change func(*settings.Settings)) error {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	next := s.SettingsSnapshot()
	change(&next)
	if s.settingsPathErr != nil {
		return s.settingsPathErr
	}
	if err := settings.SaveTo(s.settingsPath, next); err != nil {
		return err
	}
	s.Mu.Lock()
	s.Settings = cloneSettings(next)
	s.Mu.Unlock()
	return nil
}

func cloneSettings(s settings.Settings) settings.Settings {
	s.RecentInputDirs = append([]string(nil), s.RecentInputDirs...)
	s.RecentOutputDirs = append([]string(nil), s.RecentOutputDirs...)
	s.RailOrder = append([]int(nil), s.RailOrder...)
	s.RailOrderIDs = append([]string(nil), s.RailOrderIDs...)
	s.RailCollapsedGroups = append([]string(nil), s.RailCollapsedGroups...)
	// The collage block holds two slices, and a shallow copy would let a worker
	// append into the UI thread's backing array.
	s.Collage.Marks = append([]settings.CollageMark(nil), s.Collage.Marks...)
	s.Collage.GridParents = append([]int(nil), s.Collage.GridParents...)
	return s
}

// MergeSettings 只提交用户相对于基准实际修改的字段，保留后台新增的历史记录。
func (s *Session) MergeSettings(base, edited settings.Settings) error {
	return s.UpdateSettings(func(current *settings.Settings) {
		mergeSettings(current, base, edited)
	})
}

// SettingsSnapshot copies the settings for a goroutine that is not the UI thread.
// Reading Session.Settings directly from a batch worker would race with a save.
func (s *Session) SettingsSnapshot() settings.Settings {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return cloneSettings(s.Settings)
}

// OutputDirFor resolves where one item's outputs belong: the configured
// directory, else next to the source, which keeps batch results findable.
//
// With preserve_folder_structure on, a file that came from inside a dropped
// folder has that folder recreated under the configured output directory. The
// fallback needs no rebuilding: the source's own directory is already the
// preserved position.
func OutputDirFor(s settings.Settings, inputPath, relPath string) string {
	if s.DefaultOutputDir == "" {
		if appcache.IsClipboardInput(s.AIModelsDir, inputPath) {
			return filepath.Join(filepath.Dir(s.AIModelsDir), "outputs")
		}
		return filepath.Dir(inputPath)
	}
	base := s.DefaultOutputDir
	if s.PreserveFolderStructure && relPath != "" {
		if dir := filepath.Dir(filepath.FromSlash(relPath)); dir != "." && dir != "" && filepath.IsLocal(dir) {
			return filepath.Join(base, dir)
		}
	}
	return base
}

// BuildOutputPath composes the destination for one item from the template.
func BuildOutputPath(s settings.Settings, inputPath, relPath, op, ext string, seq int) string {
	ext = paths.SafeName(strings.TrimPrefix(ext, "."))
	naming := paths.Naming{Prefix: s.OutputPrefix, Template: s.OutputTemplate}
	now := time.Now()
	stem := naming.Render(inputPath, op, seq, now.Year(), int(now.Month()), now.Day(), now.Hour(), now.Minute(), now.Second())
	dir := OutputDirFor(s, inputPath, relPath)
	// Keep a suffix only when it is the encoder's own format. A stem such as
	// "photo.2024" (from "photo.2024.jpg") used to look like it already had an
	// extension, so the output was written without one and the encoder rejected
	// it as unsupported.
	name := stem
	if !strings.EqualFold(filepath.Ext(name), "."+ext) {
		name += "." + ext
	}
	target := filepath.Join(dir, name)
	root := s.DefaultOutputDir
	if root == "" {
		root = OutputDirFor(s, inputPath, "")
	}
	if err := paths.CheckContained(root, target); err != nil {
		return ""
	}
	return target
}

// FailFromResult converts an engine failure into a row status.
func FailFromResult(r engine.Result) (string, string) {
	msg := r.Error
	if msg == "" {
		msg = r.ErrorCode
	}
	if msg == "" {
		msg = "处理失败"
	}
	return "failed", msg
}
