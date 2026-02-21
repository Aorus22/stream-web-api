package js_executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// Service handles JavaScript execution via Node.js subprocess
type Service struct {
	nodePath string
	timeout  time.Duration
}

// NewService creates a new JS executor service
func NewService() *Service {
	return &Service{
		nodePath: "node", // Assumes node is in PATH
		timeout:  30 * time.Second,
	}
}

// ExecuteResult represents the result of JavaScript execution
type ExecuteResult struct {
	Result interface{} `json:"result"`
	Error  string      `json:"error,omitempty"`
}

// Execute runs JavaScript code with the given URL parameter
func (s *Service) Execute(ctx context.Context, code, url, pageType string) (*ExecuteResult, error) {
	// Get the current working directory first
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Create the wrapper script in a temp file within the working directory
	// This ensures Node.js can find node_modules
	tempFile, err := os.CreateTemp(workingDir, "js-executor-*.js")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	scriptPath := tempFile.Name()
	wrapperScript := s.buildWrapperScript(code, pageType)

	if err := os.WriteFile(scriptPath, []byte(wrapperScript), 0644); err != nil {
		return nil, fmt.Errorf("failed to write script file: %w", err)
	}

	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Execute Node.js script with ARG_FULL_URL and ARG_PAGE_TYPE
	// Set the working directory to access node_modules
	cmd := exec.CommandContext(execCtx, s.nodePath, scriptPath, url, pageType)
	cmd.Dir = workingDir

	// Capture both stdout and stderr for debugging
	var stdout, stderr []byte
	stdout, err = cmd.Output()
	if err != nil {
		// Try to get stderr for more details
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = exitErr.Stderr
		}
	}

	// Include stderr in result for debugging (only if there's content)
	if len(stderr) > 0 {
		return &ExecuteResult{
			Result: nil,
			Error:  fmt.Sprintf("Stderr: %s", string(stderr)),
		}, nil
	}

	// Check if it's a timeout
	if execCtx.Err() == context.DeadlineExceeded {
		return &ExecuteResult{
			Error: "Execution timeout - script took too long to complete",
		}, nil
	}

	// Check for other errors
	if err != nil {
		return &ExecuteResult{
			Error: fmt.Sprintf("Execution failed: %v", err),
		}, nil
	}

	// Parse the JSON output
	var result ExecuteResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return &ExecuteResult{
			Error: fmt.Sprintf("Failed to parse output: %v\nRaw output: %s", err, string(stdout)),
		}, nil
	}

	return &result, nil
}

// buildWrapperScript creates the Node.js wrapper script
func (s *Service) buildWrapperScript(userCode, pageType string) string {
	// userCode is already decoded from base64, just inject it directly
	return fmt.Sprintf("const ARG_FULL_URL = process.argv[2];\n"+
		"const ARG_PAGE_TYPE = process.argv[3] || 'list';\n"+
		"\n"+
		"(async () => {\n"+
		"  try {\n"+
		"    const AsyncFunction = Object.getPrototypeOf(async function(){}).constructor;\n"+
		"    const userFunction = new AsyncFunction('ARG_FULL_URL', 'ARG_PAGE_TYPE', 'require', 'module', %s);\n"+
		"    const result = await userFunction(ARG_FULL_URL, ARG_PAGE_TYPE, require, module);\n"+
		"    process.stdout.write(JSON.stringify({ result }));\n"+
		"    process.exit(0);\n"+
		"  } catch (error) {\n"+
		"    process.stderr.write('ERROR: ' + error.message + '\\n');\n"+
		"    process.stderr.write('STACK: ' + error.stack + '\\n');\n"+
		"    process.stdout.write(JSON.stringify({\n"+
		"      result: null,\n"+
		"      error: error.message || String(error)\n"+
		"    }));\n"+
		"    process.exit(0);\n"+
		"  }\n"+
		"})();\n", strconv.Quote(userCode))
}
