package utils

import (
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

	for e := range errCh {
		t.Fatalf("unexpected execution error: %v", e)
	}
}

