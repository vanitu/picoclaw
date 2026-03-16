package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// Client is an A2A protocol client for calling remote agents.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new A2A client with default timeout.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientWithTimeout creates a new A2A client with custom timeout.
func NewClientWithTimeout(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// FetchAgentCard fetches the agent card from the remote endpoint.
func (c *Client) FetchAgentCard(endpoint string) (*AgentCard, error) {
	url := fmt.Sprintf("%s/.well-known/agent.json", strings.TrimSuffix(endpoint, "/"))

	logger.DebugCF("a2a", "Fetching agent card", map[string]any{
		"url": url,
	})

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agent card fetch failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("failed to decode agent card: %w", err)
	}

	return &card, nil
}

// SendTask sends a task to a remote agent and returns the created task.
func (c *Client) SendTask(endpoint, token string, message Message) (*Task, error) {
	url := fmt.Sprintf("%s/a2a", strings.TrimSuffix(endpoint, "/"))

	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  MethodSendMessage,
		Params:  mustMarshal(SendMessageRequest{Message: message}),
		ID:      uuid.New().String(),
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	logger.DebugCF("a2a", "Sending task to agent", map[string]any{
		"endpoint": endpoint,
		"method":   MethodSendMessage,
	})

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send task: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("A2A error: %s (code: %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	resultBytes, err := json.Marshal(rpcResp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var sendResp SendMessageResponse
	if err := json.Unmarshal(resultBytes, &sendResp); err != nil {
		return nil, fmt.Errorf("failed to decode send message response: %w", err)
	}

	if sendResp.Task == nil {
		return nil, fmt.Errorf("no task returned")
	}

	return sendResp.Task, nil
}

// GetTask retrieves the current status of a task.
func (c *Client) GetTask(endpoint, token, taskID string) (*Task, error) {
	url := fmt.Sprintf("%s/a2a", strings.TrimSuffix(endpoint, "/"))

	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  MethodGetTask,
		Params:  mustMarshal(GetTaskRequest{TaskID: taskID}),
		ID:      uuid.New().String(),
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	logger.DebugCF("a2a", "Getting task status", map[string]any{
		"endpoint": endpoint,
		"task_id":  taskID,
	})

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("A2A error: %s (code: %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	resultBytes, err := json.Marshal(rpcResp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var task Task
	if err := json.Unmarshal(resultBytes, &task); err != nil {
		return nil, fmt.Errorf("failed to decode task: %w", err)
	}

	return &task, nil
}

// PollTask polls a task until completion or timeout.
func (c *Client) PollTask(ctx context.Context, endpoint, token, taskID string, pollInterval time.Duration) (*Task, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			task, err := c.GetTask(endpoint, token, taskID)
			if err != nil {
				return nil, err
			}

			if task.IsTerminal() {
				return task, nil
			}
		}
	}
}

// mustMarshal marshals v to JSON, panicking on error (use only for known-good values).
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
