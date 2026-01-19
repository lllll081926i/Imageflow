package utils

import (
	"fmt"
	"time"
)

type PythonExecutorPool struct {
	logger    *Logger
	executors []*PythonExecutor
	ch        chan *PythonExecutor
}

func NewPythonExecutorPool(scriptsDir string, logger *Logger, size int) (*PythonExecutorPool, error) {
	if size < 1 {
		size = 1
	}
	if size > 32 {
		size = 32
	}

	executors := make([]*PythonExecutor, 0, size)
	ch := make(chan *PythonExecutor, size)

	for i := 0; i < size; i++ {
		exec, err := NewPythonExecutor(scriptsDir, logger)
		if err != nil {
			for _, e := range executors {
				e.StopWorker()
			}
			return nil, err
		}
		executors = append(executors, exec)
		ch <- exec
	}

	return &PythonExecutorPool{
		logger:    logger,
		executors: executors,
		ch:        ch,
	}, nil
}

func (p *PythonExecutorPool) SetTimeout(timeout time.Duration) {
	for _, e := range p.executors {
		e.SetTimeout(timeout)
	}
}

func (p *PythonExecutorPool) StopWorker() {
	for _, e := range p.executors {
		e.StopWorker()
	}
}

func (p *PythonExecutorPool) Execute(scriptName string, input interface{}) ([]byte, error) {
	exec := <-p.ch
	defer func() { p.ch <- exec }()
	return exec.Execute(scriptName, input)
}

func (p *PythonExecutorPool) ExecuteAndParse(scriptName string, input interface{}, result interface{}) error {
	exec := <-p.ch
	defer func() { p.ch <- exec }()
	return exec.ExecuteAndParse(scriptName, input, result)
}

func (p *PythonExecutorPool) Size() int {
	return len(p.executors)
}

func (p *PythonExecutorPool) String() string {
	return fmt.Sprintf("PythonExecutorPool(size=%d)", len(p.executors))
}

