package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/imageflow/backend/models"
)

func TestExtractEmbeddedPythonScriptsUsesCacheRoot(t *testing.T) {
	tmpRoot := t.TempDir()
	t.Setenv(embeddedCacheRootEnv, tmpRoot)

	embedded := fstest.MapFS{
		"python/converter.py": &fstest.MapFile{Data: []byte("print('ok')")},
		"python/worker.py":    &fstest.MapFile{Data: []byte("print('worker')")},
	}

	dest, err := ExtractEmbeddedPythonScripts(embedded, "python")
	if err != nil {
		t.Fatalf("ExtractEmbeddedPythonScripts returned error: %v", err)
	}

	expected := filepath.Join(tmpRoot, "python")
	if filepath.Clean(dest) != filepath.Clean(expected) {
		t.Fatalf("expected cache extraction path %s, got %s", expected, dest)
	}

	if _, err := os.Stat(filepath.Join(dest, "converter.py")); err != nil {
		t.Fatalf("expected extracted converter.py at %s: %v", dest, err)
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Clean(filepath.Dir(exe))
		if strings.HasPrefix(filepath.Clean(dest), exeDir+string(os.PathSeparator)) || filepath.Clean(dest) == exeDir {
			t.Fatalf("expected scripts not extracted into executable directory, got %s", dest)
		}
	}
}

func TestHasEmbeddedPythonRuntime(t *testing.T) {
	tests := []struct {
		name     string
		embedded fstest.MapFS
		want     bool
	}{
		{
			name: "missing runtime payload",
			embedded: fstest.MapFS{
				"python/converter.py": &fstest.MapFile{Data: []byte("print('ok')")},
			},
			want: false,
		},
		{
			name: "random file is not enough",
			embedded: fstest.MapFS{
				"embedded_python_runtime/readme.txt": &fstest.MapFile{Data: []byte("hello")},
			},
			want: false,
		},
		{
			name: "has runtime payload",
			embedded: fstest.MapFS{
				"embedded_python_runtime/python.exe": &fstest.MapFile{Data: []byte("exe")},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasEmbeddedPythonRuntime(tt.embedded, "embedded_python_runtime")
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestNormalizeSettings_PreservesExplicitFalseFolderStructure(t *testing.T) {
	input := models.AppSettings{
		MaxConcurrency:          2,
		OutputPrefix:            "",
		OutputTemplate:          "",
		PreserveFolderStructure: false,
		ConflictStrategy:        "",
	}

	got := normalizeSettings(input)
	if got.PreserveFolderStructure {
		t.Fatal("expected preserve_folder_structure=false to remain unchanged")
	}
}

func TestNewLogger_FallsBackToConsoleWhenFileLoggingUnavailable(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmpDir)

	blockingPath := filepath.Join(tmpDir, "ImageFlow")
	if err := os.WriteFile(blockingPath, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("failed to create blocking cache path: %v", err)
	}

	logger, err := NewLogger(ErrorLevel, true)
	if err != nil {
		t.Fatalf("expected logger fallback without error, got %v", err)
	}
	if logger == nil {
		t.Fatal("expected logger instance")
	}
	if logger.file != nil {
		t.Fatal("expected file logger to be disabled when logs path is unavailable")
	}
}
