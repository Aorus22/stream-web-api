package repository

import "context"

type ScriptExecuteResult struct {
	Result interface{} `json:"result"`
	Error  string      `json:"error,omitempty"`
}

type ScriptExecutor interface {
	Execute(ctx context.Context, code, url, pageType string) (*ScriptExecuteResult, error)
}
