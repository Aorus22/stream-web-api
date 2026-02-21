package script_executor

import (
	"context"
	"errors"
)

// Service acts as the facade/router for script execution
type Service struct {
	luaExecutor *LuaExecutor
}

// NewService creates a new service with initialized executors
func NewService() *Service {
	return &Service{
		luaExecutor: &LuaExecutor{},
	}
}

// ExecuteResult represents the result of execution
type ExecuteResult struct {
	Result interface{} `json:"result"`
	Error  string      `json:"error,omitempty"`
}

// Execute determines the correct executor and runs the code strictly based on language
func (s *Service) Execute(ctx context.Context, code, url, pageType, language string) (*ExecuteResult, error) {
	if language == "lua" {
		return s.luaExecutor.Run(ctx, code, url, pageType)
	}
	
	if language == "javascript" {
		return nil, errors.New("JavaScript execution is no longer supported on this server. Please use Lua.")
	}
	
	// Default to Lua or error
	return nil, errors.New("unsupported script language: " + language)
}
