package utils

import "time"

type PythonRunner interface {
	SetTimeout(timeout time.Duration)
	StartWorker() error
	Execute(scriptName string, input interface{}) ([]byte, error)
	ExecuteAndParse(scriptName string, input interface{}, result interface{}) error
	StopWorker()
}
