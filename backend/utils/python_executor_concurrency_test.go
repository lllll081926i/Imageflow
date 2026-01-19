package utils

import (
	"strings"
	"sync"
	"testing"

	"github.com/imageflow/backend/models"
)

func TestPythonExecutor_ConcurrentExecute(t *testing.T) {
	logger, err := NewLogger(ErrorLevel, false)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	scriptsDir, err := ResolvePythonScriptsDir()
	if err != nil {
		t.Fatalf("failed to resolve scripts dir: %v", err)
	}

	executor, err := NewPythonExecutor(scriptsDir, logger)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer executor.StopWorker()

	var wg sync.WaitGroup
	errCh := make(chan error, 32)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var result models.ConvertResult
			e := executor.ExecuteAndParse("converter.py", models.ConvertRequest{
				InputPath:  "nonexistent.jpg",
				OutputPath: "output.png",
				Format:     "png",
				Quality:    90,
			}, &result)
			if e != nil {
				errCh <- e
			}
		}()
	}

	wg.Wait()
	close(errCh)

	var gotAny bool
	for e := range errCh {
		gotAny = true
		msg := e.Error()
		bad := []string{
			"__conda_tmp_",
			"used by another process",
			"exit status 3",
			"conda.cli.main_run",
		}
		for _, b := range bad {
			if strings.Contains(msg, b) {
				t.Fatalf("unexpected concurrency-related error: %v", e)
			}
		}
	}
	if !gotAny {
		t.Fatalf("expected errors for nonexistent input, got none")
	}
}
