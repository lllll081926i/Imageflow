package utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type PythonExecutor struct {
	pythonCmd  string
	pythonArgs []string
	scriptsDir string
	logger     *Logger
	timeout    time.Duration

	mu            sync.Mutex
	workerCmd     *exec.Cmd
	workerStdin   io.WriteCloser
	workerStdout  *bufio.Reader
	workerDone    chan struct{}
	workerRunning bool
}

func NewPythonExecutor(scriptsDir string, logger *Logger) (*PythonExecutor, error) {
	pythonCmd, pythonArgs, display, err := findPython()
	if err != nil {
		return nil, fmt.Errorf("failed to find Python: %w", err)
	}

	logger.Info("Found Python at: %s", display)

	return &PythonExecutor{
		pythonCmd:  pythonCmd,
		pythonArgs: pythonArgs,
		scriptsDir: scriptsDir,
		logger:     logger,
		timeout:    60 * time.Second,
	}, nil
}

func (e *PythonExecutor) SetTimeout(timeout time.Duration) {
	e.timeout = timeout
}

func (e *PythonExecutor) StartWorker() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.startWorkerLocked()
}

func (e *PythonExecutor) startWorkerLocked() error {
	if e.workerRunning {
		return nil
	}

	scriptPath := filepath.Join(e.scriptsDir, "worker.py")
	args := append([]string{}, e.pythonArgs...)
	args = append(args, scriptPath)

	cmd := exec.Command(e.pythonCmd, args...)
	cmd.Env = append(os.Environ(), "PYTHONUTF8=1", "PYTHONIOENCODING=utf-8")
	applyHideWindow(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("[PY_WORKER_START_STDIN] failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("[PY_WORKER_START_STDOUT] failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("[PY_WORKER_START_STDERR] failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("[PY_WORKER_START_FAILED] failed to start worker process: %w", err)
	}

	e.workerCmd = cmd
	e.workerStdin = stdin
	e.workerStdout = bufio.NewReader(stdout)
	e.workerDone = make(chan struct{})

	go func() {
		r := bufio.NewReader(stderr)
		for {
			line, err := r.ReadBytes('\n')
			if len(line) > 0 {
				logText := string(bytes.TrimSpace(line))
				if strings.Contains(logText, "INFO") {
					e.logger.Info("[Worker] %s", logText)
				} else if strings.Contains(logText, "WARNING") {
					e.logger.Info("[Worker Warning] %s", logText) // Use Info for warning to avoid error level noise
				} else {
					e.logger.Error("[Worker] %s", logText)
				}
			}
			if err != nil {
				return
			}
		}
	}()

	startLine, err := e.readLineLocked(10 * time.Second)
	if err != nil {
		return err
	}
	var status map[string]string
	if err := json.Unmarshal(startLine, &status); err != nil || status["status"] != "ready" {
		e.stopWorkerLocked()
		return fmt.Errorf("[PY_WORKER_START_BAD_HANDSHAKE] unexpected worker startup message: %s", string(startLine))
	}

	e.workerRunning = true

	workerDone := e.workerDone
	go func() {
		_ = cmd.Wait()
		close(workerDone)
		e.mu.Lock()
		e.workerRunning = false
		e.mu.Unlock()
	}()

	return nil
}

func (e *PythonExecutor) StopWorker() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stopWorkerLocked()
}

func (e *PythonExecutor) stopWorkerLocked() {
	if e.workerCmd == nil {
		return
	}

	if e.workerStdin != nil {
		shutdownCmd := map[string]string{"command": "shutdown"}
		jsonBytes, _ := json.Marshal(shutdownCmd)
		_, _ = e.workerStdin.Write(append(jsonBytes, '\n'))
		_ = e.workerStdin.Close()
	}

	select {
	case <-e.workerDone:
	case <-time.After(2 * time.Second):
		if e.workerCmd.Process != nil {
			_ = e.workerCmd.Process.Kill()
		}
		select {
		case <-e.workerDone:
		case <-time.After(2 * time.Second):
		}
	}

	e.workerRunning = false
	e.workerCmd = nil
	e.workerStdin = nil
	e.workerStdout = nil
	e.workerDone = nil
}

func (e *PythonExecutor) Execute(scriptName string, input interface{}) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.startWorkerLocked(); err != nil {
		return nil, err
	}

	out, workerErr, err := e.executeOnceLocked(scriptName, input)
	if err == nil {
		return out, nil
	}
	if !workerErr {
		return nil, err
	}

	e.stopWorkerLocked()
	if err := e.startWorkerLocked(); err != nil {
		return nil, err
	}
	out, _, err = e.executeOnceLocked(scriptName, input)
	return out, err
}

func (e *PythonExecutor) ExecuteAndParse(scriptName string, input interface{}, result interface{}) error {
	output, err := e.Execute(scriptName, input)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(output, result); err != nil {
		return fmt.Errorf("[PY_BAD_OUTPUT] failed to parse output: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func (e *PythonExecutor) executeOnceLocked(scriptName string, input interface{}) ([]byte, bool, error) {
	if !e.workerRunning || e.workerCmd == nil || e.workerStdin == nil || e.workerStdout == nil {
		return nil, true, fmt.Errorf("[PY_WORKER_NOT_RUNNING] python worker is not running")
	}

	cmd := map[string]interface{}{
		"script": scriptName,
		"input":  input,
	}

	inputJSON, err := json.Marshal(cmd)
	if err != nil {
		return nil, false, fmt.Errorf("[PY_BAD_INPUT] failed to marshal input: %w", err)
	}

	if _, err := e.workerStdin.Write(append(inputJSON, '\n')); err != nil {
		return nil, true, fmt.Errorf("[PY_WORKER_WRITE_FAILED] failed to write to worker: %w", err)
	}

	line, err := e.readLineLocked(e.timeout)
	if err != nil {
		return nil, true, err
	}

	return line, false, nil
}

func (e *PythonExecutor) readLineLocked(timeout time.Duration) ([]byte, error) {
	type readResult struct {
		line []byte
		err  error
	}
	ch := make(chan readResult, 1)

	go func() {
		line, err := e.workerStdout.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimSpace(line)
		}
		ch <- readResult{line: line, err: err}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			e.stopWorkerLocked()
			return nil, fmt.Errorf("[PY_WORKER_READ_FAILED] failed to read from worker: %w", res.err)
		}
		if len(res.line) == 0 {
			return nil, fmt.Errorf("[PY_WORKER_NO_OUTPUT] worker produced no output")
		}
		return res.line, nil
	case <-time.After(timeout):
		e.stopWorkerLocked()
		return nil, fmt.Errorf("[PY_WORKER_TIMEOUT] execution timed out after %v", timeout)
	}
}

func findPython() (string, []string, string, error) {
	if v := os.Getenv("IMAGEFLOW_PYTHON_EXE"); v != "" {
		if !filepath.IsAbs(v) {
			abs, err := filepath.Abs(v)
			if err == nil {
				v = abs
			}
		}
		if isPython3Executable(v) {
			return v, nil, v, nil
		}
		return "", nil, "", fmt.Errorf("IMAGEFLOW_PYTHON_EXE is set but not a Python 3 executable: %s", v)
	}

	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 6; i++ {
			for _, candidate := range venvPythonCandidates(dir) {
				if isPython3Executable(candidate) {
					return candidate, nil, candidate, nil
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		for _, candidate := range bundledPythonCandidates(exeDir) {
			if isPython3Executable(candidate) {
				return candidate, nil, candidate, nil
			}
		}
	}

	// Fallback to regular Python search
	candidates := []string{"python3", "python"}

	// On Windows, also try with .exe extension
	if runtime.GOOS == "windows" {
		candidates = append(candidates, "python3.exe", "python.exe")
	}

	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate)
		if err == nil {
			// Verify it's Python 3
			if isPython3Executable(path) {
				return path, nil, path, nil
			}
		}
	}

	condaPath, err := exec.LookPath("conda")
	if err == nil {
		args := []string{"run", "--no-capture-output", "-n", "imageflow", "python"}

		resolveCmd := exec.Command(condaPath, append(args, "-c", "import sys; print(sys.executable)")...)
		applyHideWindow(resolveCmd)
		resolveOut, resolveErr := resolveCmd.Output()
		if resolveErr == nil {
			pythonExe := strings.TrimSpace(string(resolveOut))
			if pythonExe != "" && isPython3Executable(pythonExe) {
				return pythonExe, nil, pythonExe, nil
			}
		}

		cmd := exec.Command(condaPath, append(args, "--version")...)
		applyHideWindow(cmd)
		output, err := cmd.Output()
		if err == nil && bytes.Contains(output, []byte("Python 3")) {
			return condaPath, args, "conda run -n imageflow python", nil
		}
	}

	return "", nil, "", fmt.Errorf("python executable not found (tried: bundled python, %v, conda imageflow)", candidates)
}

func isPython3Executable(pythonPath string) bool {
	cmd := exec.Command(pythonPath, "--version")
	applyHideWindow(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	return bytes.Contains(output, []byte("Python 3"))
}

func venvPythonCandidates(baseDir string) []string {
	if runtime.GOOS == "windows" {
		return []string{
			filepath.Join(baseDir, ".venv", "Scripts", "python.exe"),
			filepath.Join(baseDir, "python", ".venv", "Scripts", "python.exe"),
		}
	}
	return []string{
		filepath.Join(baseDir, ".venv", "bin", "python3"),
		filepath.Join(baseDir, ".venv", "bin", "python"),
		filepath.Join(baseDir, "python", ".venv", "bin", "python3"),
		filepath.Join(baseDir, "python", ".venv", "bin", "python"),
	}
}

func bundledPythonCandidates(baseDir string) []string {
	if runtime.GOOS == "windows" {
		return []string{
			filepath.Join(baseDir, "python", "python.exe"),
			filepath.Join(baseDir, "python", "python3.exe"),
			filepath.Join(baseDir, "python_runtime", "python.exe"),
			filepath.Join(baseDir, "python.exe"),
		}
	}
	return []string{
		filepath.Join(baseDir, "python", "bin", "python3"),
		filepath.Join(baseDir, "python", "bin", "python"),
		filepath.Join(baseDir, "python3"),
		filepath.Join(baseDir, "python"),
	}
}
