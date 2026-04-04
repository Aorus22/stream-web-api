package usecase

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	domainrepo "stream-web-api/internal/domain/repository"
)

type ScriptExecutorUsecase struct {
	executor domainrepo.ScriptExecutor
}

func NewScriptExecutorUsecase(executor domainrepo.ScriptExecutor) *ScriptExecutorUsecase {
	return &ScriptExecutorUsecase{executor: executor}
}

func (s *ScriptExecutorUsecase) Execute(ctx context.Context, code, url, pageType, language string) (*domainrepo.ScriptExecuteResult, error) {
	if language == "lua" {
		return s.executor.Execute(ctx, code, url, pageType)
	}
	if language == "javascript" {
		return &domainrepo.ScriptExecuteResult{Error: "JavaScript execution is no longer supported on this server. Please use Lua."}, nil
	}
	return &domainrepo.ScriptExecuteResult{Error: "unsupported script language: " + language}, nil
}

func (s *ScriptExecutorUsecase) ExecuteWithBase64(ctx context.Context, codeB64, url string, pageType, language string, isBase64 bool) (*domainrepo.ScriptExecuteResult, error) {
	code := codeB64
	if isBase64 && code != "" {
		decoded, err := base64.StdEncoding.DecodeString(code)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 code: %w", err)
		}
		code = string(decoded)
	}

	if code == "" {
		return nil, fmt.Errorf("code is required")
	}

	if url == "" {
		return nil, fmt.Errorf("url is required")
	}

	if pageType == "" {
		pageType = "list"
	}

	if language == "" {
		language = "javascript"
	}

	return s.Execute(ctx, code, url, pageType, language)
}

func (s *ScriptExecutorUsecase) FetchURLContent(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}
