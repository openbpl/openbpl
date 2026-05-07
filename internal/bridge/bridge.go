// Package bridge manages the Node.js sidecar process that executes TypeScript rules.
package bridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Label is a detection result from a TypeScript rule.
type Label struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
	Detail     string  `json:"detail"`
}

// Evidence is the data sent to the Node.js runtime for evaluation.
type Evidence struct {
	Domain         string `json:"domain"`
	HTML           string `json:"html"`
	Title          string `json:"title"`
	ScreenshotPath string `json:"screenshotPath"`
	Screenshot     string `json:"screenshot"`
}

// Brand is the brand config sent to the Node.js runtime.
type Brand struct {
	Name        string   `json:"name"`
	Website     string   `json:"website"`
	Description string   `json:"description"`
	Industry    string   `json:"industry"`
	Keywords    Keywords `json:"keywords"`
	Images      []string `json:"images"`
	Colors      []string `json:"colors"`
	URLs        URLs     `json:"urls"`
}

type Keywords struct {
	Included []string `json:"included"`
	Excluded []string `json:"excluded"`
}

type URLs struct {
	Domains           []string `json:"domains"`
	SocialMedia       []string `json:"socialMedia"`
	AppStores         []string `json:"appStores"`
	BrowserExtensions []string `json:"browserExtensions"`
	Blogs             []string `json:"blogs"`
}

// EvaluateParams is the JSON-RPC params for the evaluate method.
type EvaluateParams struct {
	Evidence Evidence `json:"evidence"`
	Brand    Brand    `json:"brand"`
}

type rpcRequest struct {
	ID     int64       `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

type rpcResponse struct {
	ID     int64            `json:"id"`
	Result json.RawMessage  `json:"result,omitempty"`
	Error  *rpcError        `json:"error,omitempty"`
}

type rpcError struct {
	Message string `json:"message"`
}

// Bridge manages communication with the Node.js sidecar.
type Bridge struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	mu     sync.Mutex
	nextID atomic.Int64
}

// Start launches the Node.js runtime process.
// rulesDir is the path to the directory containing TypeScript rule files.
// runtimePath is the path to the compiled runtime.js entry point.
func Start(runtimePath, rulesDir string) (*Bridge, error) {
	cmd := exec.Command("node", "--experimental-strip-types", runtimePath, rulesDir)
	cmd.Stderr = nil // Let Node.js stderr go to the Go process stderr for logging

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start node: %w", err)
	}

	b := &Bridge{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
	}

	// Wait for ready signal
	if !b.stdout.Scan() {
		cmd.Process.Kill()
		return nil, fmt.Errorf("node runtime did not send ready signal")
	}

	var ready rpcResponse
	if err := json.Unmarshal(b.stdout.Bytes(), &ready); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("parse ready signal: %w", err)
	}

	if ready.Error != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("node runtime error: %s", ready.Error.Message)
	}

	// Parse the ready result to log how many rules were loaded
	var readyResult struct {
		Ready bool `json:"ready"`
		Rules int  `json:"rules"`
	}
	if err := json.Unmarshal(ready.Result, &readyResult); err == nil {
		log.Printf("node bridge: %d rules loaded", readyResult.Rules)
	}

	return b, nil
}

// Evaluate sends evidence to the Node.js runtime and returns labels.
func (b *Bridge) Evaluate(params EvaluateParams) ([]Label, error) {
	result, err := b.call("evaluate", params)
	if err != nil {
		return nil, err
	}

	var labels []Label
	if err := json.Unmarshal(result, &labels); err != nil {
		return nil, fmt.Errorf("parse labels: %w", err)
	}
	return labels, nil
}

// List returns the names of all loaded rules.
func (b *Bridge) List() ([]struct{ Name, Description string }, error) {
	result, err := b.call("list", nil)
	if err != nil {
		return nil, err
	}

	var rules []struct{ Name, Description string }
	if err := json.Unmarshal(result, &rules); err != nil {
		return nil, fmt.Errorf("parse rule list: %w", err)
	}
	return rules, nil
}

// Stop shuts down the Node.js process.
func (b *Bridge) Stop() error {
	b.stdin.Close()
	return b.cmd.Wait()
}

func (b *Bridge) call(method string, params interface{}) (json.RawMessage, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID.Add(1)
	req := rpcRequest{
		ID:     id,
		Method: method,
		Params: params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	if _, err := b.stdin.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("write to node: %w", err)
	}

	if !b.stdout.Scan() {
		return nil, fmt.Errorf("node process closed stdout")
	}

	var resp rpcResponse
	if err := json.Unmarshal(b.stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("node error: %s", resp.Error.Message)
	}

	return resp.Result, nil
}
