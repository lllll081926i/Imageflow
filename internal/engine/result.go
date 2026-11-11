package engine

// Result is the envelope every engine returns to the UI and the batch runner.
type Result struct {
	Success    bool     `json:"success"`
	Error      string   `json:"error,omitempty"`
	ErrorCode  string   `json:"error_code,omitempty"`
	OutputPath string   `json:"output_path,omitempty"`
	OutputDir  string   `json:"output_dir,omitempty"`
	Outputs    []string `json:"outputs,omitempty"`
	Width      int      `json:"width,omitempty"`
	Height     int      `json:"height,omitempty"`
	InputSize  int64    `json:"input_size,omitempty"`
	OutputSize int64    `json:"output_size,omitempty"`
	Count      int      `json:"count,omitempty"`
	Warning    string   `json:"warning,omitempty"`
	Message    string   `json:"message,omitempty"`
	// Skipped marks a row the runner chose not to write: the destination exists
	// and the conflict strategy is 跳过. It is neither a success nor a failure,
	// so the runner keeps it out of both counters instead of reporting an
	// overwrite that did not happen.
	Skipped bool `json:"skipped,omitempty"`
}

// Fail builds a failure envelope carrying a stable error code.
func Fail(code, message string) Result {
	return Result{Success: false, ErrorCode: code, Error: message}
}

// OK builds a success envelope.
func OK() Result { return Result{Success: true} }

// Skip builds the envelope for a row the conflict strategy left alone.
func Skip(reason string) Result {
	return Result{Success: true, Skipped: true, Message: reason}
}
