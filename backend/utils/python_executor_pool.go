package utils

import (
	"fmt"
	"sync/atomic"
	"time"
)

type PythonExecutorPool struct {
	logger         *Logger
	executors      []*PythonExecutor
	ch             chan *PythonExecutor
	acquireTimeout time.Duration
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
		logger:         logger,
		executors:      executors,
		ch:             ch,
		acquireTimeout: 60 * time.Second,
	}, nil
}

func (p *PythonExecutorPool) SetTimeout(timeout time.Duration) {
	if timeout > 0 {
		p.acquireTimeout = timeout
	}
	for _, e := range p.executors {
		e.SetTimeout(timeout)
	}
}

func (p *PythonExecutorPool) StartWorker() error {
	var firstErr error
	for _, e := range p.executors {
		if err := e.StartWorker(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (p *PythonExecutorPool) StopWorker() {
	for _, e := range p.executors {
		e.StopWorker()
	}
}

func (p *PythonExecutorPool) CancelActiveTask() {
	for _, e := range p.executors {
		if e == nil || atomic.LoadUint32(&e.taskRunning) == 0 {
			continue
		}
		e.CancelActiveTask()
	}
}

func (p *PythonExecutorPool) Execute(scriptName string, input interface{}) (output []byte, err error) {
	exec, err := p.acquireExecutor()
	if err != nil {
		return nil, err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("python executor panic: %v", recovered)
		}
		p.releaseExecutor(exec)
	}()
	output, err = exec.Execute(scriptName, input)
	return output, err
}

func (p *PythonExecutorPool) ExecuteAndParse(scriptName string, input interface{}, result interface{}) (err error) {
	exec, err := p.acquireExecutor()
	if err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("python executor panic: %v", recovered)
		}
		p.releaseExecutor(exec)
	}()
	err = exec.ExecuteAndParse(scriptName, input, result)
	return err
}

func (p *PythonExecutorPool) Size() int {
	return len(p.executors)
}

func (p *PythonExecutorPool) String() string {
	return fmt.Sprintf("PythonExecutorPool(size=%d)", len(p.executors))
}

func (p *PythonExecutorPool) acquireExecutor() (*PythonExecutor, error) {
	timeout := p.acquireTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case exec := <-p.ch:
		if exec == nil {
			return nil, fmt.Errorf("python executor pool returned nil executor")
		}
		return exec, nil
	case <-timer.C:
		return nil, fmt.Errorf("timed out waiting for python executor after %v", timeout)
	}
}

func (p *PythonExecutorPool) releaseExecutor(exec *PythonExecutor) {
	if exec == nil {
		return
	}
	select {
	case p.ch <- exec:
	default:
		if p.logger != nil {
			p.logger.Error("Python executor pool release overflow; stopping leaked executor")
		}
		exec.StopWorker()
	}
}
