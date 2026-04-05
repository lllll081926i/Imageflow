package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	resultCh := make(chan models.ConvertResult, 32)

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
				return
			}
			resultCh <- result
		}()
	}

	wg.Wait()
	close(errCh)
	close(resultCh)

	for e := range errCh {
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

	var gotFailureResult bool
	for r := range resultCh {
		if !r.Success {
			gotFailureResult = true
			if r.Error == "" {
				t.Fatalf("expected error message in result, got empty")
			}
		}
	}
	if !gotFailureResult {
		t.Fatalf("expected failure results for nonexistent input, got none")
	}
}

func TestPythonExecutorPool_ExecuteTimesOutWhenNoExecutorAvailable(t *testing.T) {
	pool := &PythonExecutorPool{
		ch:             make(chan *PythonExecutor),
		acquireTimeout: 20 * time.Millisecond,
	}

	start := time.Now()
	_, err := pool.Execute("converter.py", nil)
	if err == nil {
		t.Fatal("expected acquire timeout error")
	}
	if !strings.Contains(err.Error(), "timed out waiting for python executor") {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
		t.Fatalf("expected Execute to wait for acquire timeout, elapsed=%v", elapsed)
	}
}

func TestResolvePython_RetriesAfterInitialFailure(t *testing.T) {
	validPython := findUsablePythonExecutable(t)

	resetResolvedPythonCacheForTests()
	t.Setenv("IMAGEFLOW_PYTHON_EXE", filepath.Join(t.TempDir(), "missing-python.exe"))
	if _, _, _, err := resolvePython(); err == nil {
		t.Fatal("expected invalid IMAGEFLOW_PYTHON_EXE to fail")
	}

	t.Setenv("IMAGEFLOW_PYTHON_EXE", validPython)
	cmd, args, display, err := resolvePython()
	if err != nil {
		t.Fatalf("expected resolvePython to retry after failure, got %v", err)
	}
	if cmd != validPython {
		t.Fatalf("expected resolved python %q, got %q", validPython, cmd)
	}
	if len(args) != 0 {
		t.Fatalf("expected direct python executable without args, got %#v", args)
	}
	if display != validPython {
		t.Fatalf("expected display %q, got %q", validPython, display)
	}
}

func TestIsPythonCancelledRecognizesSentinelAndLegacyMessage(t *testing.T) {
	if !IsPythonCancelled(fmt.Errorf("wrapped: %w", ErrPythonCancelled)) {
		t.Fatal("expected wrapped sentinel cancellation error to be recognized")
	}
	if !IsPythonCancelled(fmt.Errorf("%s", "[PY_CANCELLED] operation cancelled")) {
		t.Fatal("expected legacy cancellation message to be recognized")
	}
	if IsPythonCancelled(fmt.Errorf("different error")) {
		t.Fatal("did not expect unrelated error to be recognized as cancellation")
	}
}

func TestPythonExecutorPool_CancelActiveTaskSkipsIdleExecutors(t *testing.T) {
	active := &PythonExecutor{}
	idle := &PythonExecutor{}
	atomic.StoreUint32(&active.taskRunning, 1)

	pool := &PythonExecutorPool{
		executors: []*PythonExecutor{active, idle},
	}
	pool.CancelActiveTask()

	if got := atomic.LoadUint32(&active.cancelFlag); got != 1 {
		t.Fatalf("expected active executor cancel flag to be set, got %d", got)
	}
	if got := atomic.LoadUint32(&idle.cancelFlag); got != 0 {
		t.Fatalf("expected idle executor cancel flag to remain clear, got %d", got)
	}
}

func TestPythonExecutorPool_ReleaseExecutorDoesNotBlockWhenChannelFull(t *testing.T) {
	logger, err := NewLogger(ErrorLevel, false)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	exec := &PythonExecutor{}
	pool := &PythonExecutorPool{
		logger: logger,
		ch:     make(chan *PythonExecutor, 1),
	}
	pool.ch <- &PythonExecutor{}

	done := make(chan struct{})
	go func() {
		pool.releaseExecutor(exec)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected releaseExecutor not to block when channel is full")
	}
}

func findUsablePythonExecutable(t *testing.T) string {
	t.Helper()

	candidates := []string{}
	if env := strings.TrimSpace(os.Getenv("IMAGEFLOW_PYTHON_EXE")); env != "" {
		candidates = append(candidates, env)
	}
	for _, candidate := range []string{"python", "python3", "python.exe", "python3.exe"} {
		if path, err := exec.LookPath(candidate); err == nil {
			candidates = append(candidates, path)
		}
	}
	if cmd, _, _, err := findPython(); err == nil {
		candidates = append(candidates, cmd)
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if isPython3Executable(candidate) {
			return candidate
		}
	}

	t.Skip("no direct python executable available for resolvePython retry test")
	return ""
}
