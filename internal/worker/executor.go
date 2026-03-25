package worker

import "net/http"

func NewMockPythonExecutor() *http.Client {
	h := new(http.Client)
	return h
}
