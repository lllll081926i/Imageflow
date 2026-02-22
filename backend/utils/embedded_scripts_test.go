package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
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
