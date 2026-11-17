package thumbs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/disintegration/imaging"

	"github.com/lllll081926i/imageflow-go/internal/codec"
)

const (
	// MaxEdge caps the thumbnail's long edge in pixels.
	MaxEdge = 256
	// MemoryEntries caps the in-memory LRU size.
	MemoryEntries = 256
	// MemoryBytes caps the total decoded bytes held in memory.
	MemoryBytes = 32 << 20
	// DiskFiles caps how many files the on-disk cache keeps.
	DiskFiles = 800
	// DiskBytes caps the total size of the on-disk cache.
	DiskBytes = 128 << 20
)

// Info is one rendered thumbnail.
type Info struct {
	Image    image.Image
	Data     []byte
	Width    int
	Height   int
	FromDisk bool
}

type memoryEntry struct {
	info *Info
	size int
}

// Cache keeps decoded thumbnails in memory and encoded ones on disk.
type Cache struct {
	mu         sync.Mutex
	order      []string // LRU keys, oldest first
	entries    map[string]*memoryEntry
	totalBytes int
	dir        string
}

// New creates a cache rooted at the user's cache directory.
func New() *Cache {
	c := &Cache{entries: map[string]*memoryEntry{}}
	base, err := os.UserCacheDir()
	if err == nil {
		dir := filepath.Join(base, "imageflow", "thumb_cache")
		if mkErr := os.MkdirAll(dir, 0o755); mkErr == nil {
			c.dir = dir
		}
	}
	return c
}

func keyFor(path string, modTimeNS, size int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d|%d|srgb-v1", path, modTimeNS, size, MaxEdge)))
	return hex.EncodeToString(sum[:])
}

// Get returns a cached thumbnail, rendering it when missing.
func (c *Cache) Get(path string, modTimeNS, size int64) (*Info, error) {
	key := keyFor(path, modTimeNS, size)
	c.mu.Lock()
	if entry, ok := c.entries[key]; ok {
		c.touchLocked(key)
		c.mu.Unlock()
		return entry.info, nil
	}
	dir := c.dir
	c.mu.Unlock()

	if dir != "" {
		if info, err := readDisk(dir, key); err == nil {
			c.remember(key, info)
			return info, nil
		}
	}

	info, err := render(path)
	if err != nil {
		return nil, err
	}
	if dir != "" {
		writeDisk(dir, key, info.Data)
	}
	c.remember(key, info)
	return info, nil
}

func render(path string) (*Info, error) {
	opened, err := codec.OpenSRGB(path)
	if err != nil {
		return nil, err
	}
	thumb := imaging.Fit(opened.Image, MaxEdge, MaxEdge, imaging.Lanczos)
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&buf, thumb); err != nil {
		return nil, err
	}
	b := thumb.Bounds()
	return &Info{Image: thumb, Data: buf.Bytes(), Width: b.Dx(), Height: b.Dy()}, nil
}

func readDisk(dir, key string) (*Info, error) {
	data, err := os.ReadFile(filepath.Join(dir, key+".png"))
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	if b.Dx() > MaxEdge || b.Dy() > MaxEdge {
		return nil, fmt.Errorf("缓存尺寸无效")
	}
	return &Info{Image: img, Data: data, Width: b.Dx(), Height: b.Dy(), FromDisk: true}, nil
}

func writeDisk(dir, key string, data []byte) {
	target := filepath.Join(dir, key+".png")
	tmp, err := os.CreateTemp(dir, ".thumb-*")
	if err != nil {
		return
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return
	}
	if err := os.Rename(name, target); err != nil {
		os.Remove(name)
		return
	}
	pruneIfNeeded(dir)
}

func pruneIfNeeded(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type fileEntry struct {
		name string
		size int64
		mod  int64
	}
	files := make([]fileEntry, 0, len(entries))
	var total int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileEntry{name: e.Name(), size: info.Size(), mod: info.ModTime().UnixNano()})
		total += info.Size()
	}
	if len(files) <= DiskFiles && total <= DiskBytes {
		return
	}
	// Oldest first, then drop until both budgets hold. The index walks the sorted
	// list once; a file that cannot be removed still advances the cursor, which
	// a slice-reslice would not do.
	sort.Slice(files, func(i, j int) bool { return files[i].mod < files[j].mod })
	remaining := len(files)
	for _, f := range files {
		if remaining <= DiskFiles && total <= DiskBytes {
			break
		}
		if err := os.Remove(filepath.Join(dir, f.name)); err == nil {
			remaining--
			total -= f.size
		}
	}
}

func (c *Cache) remember(key string, info *Info) {
	size := len(info.Data)
	if size <= 0 || size > 8<<20 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// A key can be remembered twice when two goroutines render the same file at
	// once; the LRU order must still hold one entry per key.
	if old, ok := c.entries[key]; ok {
		c.totalBytes -= old.size
		delete(c.entries, key)
		for i, k := range c.order {
			if k == key {
				c.order = append(c.order[:i], c.order[i+1:]...)
				break
			}
		}
	}
	c.entries[key] = &memoryEntry{info: info, size: size}
	c.order = append(c.order, key)
	c.totalBytes += size
	for len(c.order) > MemoryEntries || c.totalBytes > MemoryBytes {
		oldest := c.order[0]
		c.order = c.order[1:]
		if entry, ok := c.entries[oldest]; ok {
			c.totalBytes -= entry.size
			delete(c.entries, oldest)
		}
	}
}

func (c *Cache) touchLocked(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(append(c.order[:i:i], c.order[i+1:]...), key)
			return
		}
	}
}

// Len reports how many thumbnails are held in memory, for tests.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

func (c *Cache) ClearMemory() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = map[string]*memoryEntry{}
	c.order = nil
	c.totalBytes = 0
}

// OrderLen reports the LRU order's length, for tests: it must equal Len, or an
// entry would be evicted out of turn.
func (c *Cache) OrderLen() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.order)
}
