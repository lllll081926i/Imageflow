package services

import (
	"strings"
	"testing"
	"time"

	"github.com/imageflow/backend/models"
	"github.com/imageflow/backend/utils"
)

type mockPythonRunner struct {
	executeAndParseFn func(scriptName string, input interface{}, result interface{}) error
}

func (m *mockPythonRunner) SetTimeout(timeout time.Duration) {}
func (m *mockPythonRunner) StartWorker() error               { return nil }
func (m *mockPythonRunner) Execute(scriptName string, input interface{}) ([]byte, error) {
	return nil, nil
}
func (m *mockPythonRunner) ExecuteAndParse(scriptName string, input interface{}, result interface{}) error {
	if m.executeAndParseFn != nil {
		return m.executeAndParseFn(scriptName, input, result)
	}
	return nil
}
func (m *mockPythonRunner) CancelActiveTask() {}
func (m *mockPythonRunner) StopWorker()       {}

func newTestLogger(t *testing.T) *utils.Logger {
	t.Helper()
	logger, err := utils.NewLogger(utils.ErrorLevel, false)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	return logger
}

func TestSplitGIF_CompressPassesQuality(t *testing.T) {
	logger := newTestLogger(t)
	defer logger.Close()

	runner := &mockPythonRunner{
		executeAndParseFn: func(scriptName string, input interface{}, result interface{}) error {
			if scriptName != "gif_splitter.py" {
				t.Fatalf("expected script gif_splitter.py, got %s", scriptName)
			}
			payload, ok := input.(map[string]interface{})
			if !ok {
				t.Fatalf("expected payload map, got %T", input)
			}
			if payload["action"] != "compress" {
				t.Fatalf("expected action compress, got %#v", payload["action"])
			}
			if payload["quality"] != 90 {
				t.Fatalf("expected quality 90, got %#v", payload["quality"])
			}
			res := result.(*models.GIFSplitResult)
			*res = models.GIFSplitResult{Success: true, Quality: 90}
			return nil
		},
	}

	service := NewGIFSplitterService(runner, logger)
	res, err := service.SplitGIF(models.GIFSplitRequest{
		Action:     "COMPRESS",
		InputPath:  "in.gif",
		OutputPath: "out.gif",
		Quality:    90,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !res.Success || res.Quality != 90 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestSplitGIF_BuildGifFallsBackToInputPath(t *testing.T) {
	logger := newTestLogger(t)
	defer logger.Close()

	runner := &mockPythonRunner{
		executeAndParseFn: func(scriptName string, input interface{}, result interface{}) error {
			payload := input.(map[string]interface{})
			paths, ok := payload["input_paths"].([]string)
			if !ok {
				t.Fatalf("expected []string input_paths, got %T", payload["input_paths"])
			}
			if len(paths) != 1 || paths[0] != "frame1.png" {
				t.Fatalf("unexpected input_paths: %#v", paths)
			}
			res := result.(*models.GIFSplitResult)
			*res = models.GIFSplitResult{Success: true}
			return nil
		},
	}

	service := NewGIFSplitterService(runner, logger)
	_, err := service.SplitGIF(models.GIFSplitRequest{
		Action:     "build_gif",
		InputPath:  "frame1.png",
		OutputPath: "out.gif",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestSplitGIF_ReturnsErrorWhenPythonResultFails(t *testing.T) {
	logger := newTestLogger(t)
	defer logger.Close()

	runner := &mockPythonRunner{
		executeAndParseFn: func(scriptName string, input interface{}, result interface{}) error {
			res := result.(*models.GIFSplitResult)
			*res = models.GIFSplitResult{Success: false, Error: "boom"}
			return nil
		},
	}

	service := NewGIFSplitterService(runner, logger)
	_, err := service.SplitGIF(models.GIFSplitRequest{
		Action:     "reverse",
		InputPath:  "in.gif",
		OutputPath: "out.gif",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "GIF processing failed: boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

