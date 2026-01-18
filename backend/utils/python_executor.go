package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// PythonExecutor handles execution of Python scripts
type PythonExecutor struct {
	pythonCmd  string
	pythonArgs []string
	scriptsDir string
	logger     *Logger
	timeout    time.Duration
	useConda   bool
	execMu     sync.Mutex
}

// NewPythonExecutor creates a new Python executor
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
		timeout:    60 * time.Second, // Default 60 second timeout
		useConda:   len(pythonArgs) > 0 && pythonArgs[0] == "run",
	}, nil
}

// SetTimeout sets the execution timeout
func (e *PythonExecutor) SetTimeout(timeout time.Duration) {
	e.timeout = timeout
}

// Execute runs a Python script with JSON input and returns JSON output
func (e *PythonExecutor) Execute(scriptName string, input interface{}) ([]byte, error) {
	// Marshal input to JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	e.logger.Debug("Executing script: %s with input: %s", scriptName, string(inputJSON))

	// Build script path
	scriptPath := filepath.Join(e.scriptsDir, scriptName)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	args := append([]string{}, e.pythonArgs...)
	args = append(args, scriptPath)
	cmd := exec.CommandContext(ctx, e.pythonCmd, args...)
	cmd.Stdin = bytes.NewReader(inputJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute command
	startTime := time.Now()
	if e.useConda {
		e.execMu.Lock()
		defer e.execMu.Unlock()
	}
	err = cmd.Run()
	duration := time.Since(startTime)

	e.logger.Debug("Script execution completed in %v", duration)

	// Check for timeout
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("script execution timed out after %v", e.timeout)
	}

	// Check for execution errors
	if err != nil {
		e.logger.Error("Script execution failed: %v", err)
		e.logger.Error("Stderr: %s", stderr.String())
		return nil, fmt.Errorf("script execution failed: %w\nStderr: %s", err, stderr.String())
	}

	// Get output
	output := stdout.Bytes()
	if len(output) == 0 {
		return nil, fmt.Errorf("script produced no output")
	}

	e.logger.Debug("Script output: %s", string(output))

	return output, nil
}

// ExecuteAndParse runs a Python script and parses the JSON result into a struct
func (e *PythonExecutor) ExecuteAndParse(scriptName string, input interface{}, result interface{}) error {
	output, err := e.Execute(scriptName, input)
	if err != nil {
		return err
	}

	// Parse JSON output
	if err := json.Unmarshal(output, result); err != nil {
		return fmt.Errorf("failed to parse output: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// findPython attempts to find a Python executable
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

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		for _, candidate := range bundledPythonCandidates(exeDir) {
			if isPython3Executable(candidate) {
				return candidate, nil, candidate, nil
			}
		}
	}

	condaPath, err := exec.LookPath("conda")
	if err == nil {
		args := []string{"run", "--no-capture-output", "-n", "imageflow", "python"}
		cmd := exec.Command(condaPath, append(args, "--version")...)
		output, err := cmd.Output()
		if err == nil && bytes.Contains(output, []byte("Python 3")) {
			return condaPath, args, "conda run -n imageflow python", nil
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

	return "", nil, "", fmt.Errorf("python executable not found (tried: bundled python, conda imageflow, %v)", candidates)
}

func isPython3Executable(pythonPath string) bool {
	cmd := exec.Command(pythonPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Check if output contains "Python 3"
	return bytes.Contains(output, []byte("Python 3"))
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
