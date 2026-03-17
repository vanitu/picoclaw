package krabot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestChannel(t *testing.T) *KrabotChannel {
	cfg := KrabotConfig{
		Token:          "test-token",
		MaxConnections: 10,
	}
	mb := bus.NewMessageBus()
	ch, err := NewKrabotChannel(cfg, mb)
	require.NoError(t, err)
	return ch
}

func TestKrabotChannel_Send(t *testing.T) {
	ch := setupTestChannel(t)

	// Start the channel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, ch.Start(ctx))

	// Use a specific session ID
	testSessionID := "test-session-123"

	// Create a test WebSocket connection with specific session
	server := httptest.NewServer(http.HandlerFunc(ch.handleWebSocket))
	defer server.Close()

	// Convert http:// to ws:// and include session_id
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=test-token&session_id=" + testSessionID

	// Connect WebSocket client
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	// Wait for connection to be established
	time.Sleep(100 * time.Millisecond)

	// Test Send - chatID must match the session ID format
	chatID := "krabot:" + testSessionID
	msg := bus.OutboundMessage{
		ChatID:  chatID,
		Content: "Hello from test!",
	}

	err = ch.Send(ctx, msg)
	require.NoError(t, err)

	// Read the message from WebSocket
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	var received KrabotMessage
	err = ws.ReadJSON(&received)
	require.NoError(t, err)

	// Verify message structure
	assert.Equal(t, TypeMessageCreate, received.Type)
	assert.Equal(t, "Hello from test!", received.Payload.Content)
	assert.NotEmpty(t, received.Payload.MessageID, "message_id should not be empty")
	assert.True(t, received.Payload.Final, "final should be true for regular messages")
	assert.Equal(t, testSessionID, received.SessionID)
	assert.NotZero(t, received.Timestamp)
}

func TestKrabotChannel_SendPlaceholder(t *testing.T) {
	ch := setupTestChannel(t)

	// Start the channel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, ch.Start(ctx))

	// Use a specific session ID
	testSessionID := "test-session-456"

	// Create a test WebSocket connection with specific session
	server := httptest.NewServer(http.HandlerFunc(ch.handleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=test-token&session_id=" + testSessionID

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	time.Sleep(100 * time.Millisecond)

	// Test SendPlaceholder
	chatID := "krabot:" + testSessionID
	msgID, err := ch.SendPlaceholder(ctx, chatID)
	require.NoError(t, err)
	require.NotEmpty(t, msgID, "SendPlaceholder should return a message ID")

	// Read the message from WebSocket
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	var received KrabotMessage
	err = ws.ReadJSON(&received)
	require.NoError(t, err)

	// Verify placeholder message structure
	assert.Equal(t, TypeMessageCreate, received.Type)
	assert.Equal(t, "Thinking... 💭", received.Payload.Content)
	assert.Equal(t, msgID, received.Payload.MessageID, "message_id should match returned ID")
	assert.False(t, received.Payload.Final, "final should be false for placeholder messages")
	assert.Equal(t, testSessionID, received.SessionID)
}

func TestKrabotChannel_EditMessage(t *testing.T) {
	ch := setupTestChannel(t)

	// Start the channel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, ch.Start(ctx))

	// Use a specific session ID
	testSessionID := "test-session-789"

	// Create a test WebSocket connection with specific session
	server := httptest.NewServer(http.HandlerFunc(ch.handleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=test-token&session_id=" + testSessionID

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	time.Sleep(100 * time.Millisecond)

	// Test EditMessage
	chatID := "krabot:" + testSessionID
	originalMsgID := "test-msg-123"
	updatedContent := "Updated message content"

	err = ch.EditMessage(ctx, chatID, originalMsgID, updatedContent)
	require.NoError(t, err)

	// Read the message from WebSocket
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	var received KrabotMessage
	err = ws.ReadJSON(&received)
	require.NoError(t, err)

	// Verify update message structure
	assert.Equal(t, TypeMessageUpdate, received.Type)
	assert.Equal(t, updatedContent, received.Payload.Content)
	assert.Equal(t, originalMsgID, received.Payload.MessageID, "message_id should match the edited message ID")
	assert.True(t, received.Payload.Final, "final should be true for updated messages")
	assert.Equal(t, testSessionID, received.SessionID)
}

func TestKrabotChannel_MessageIDInPayload(t *testing.T) {
	// Test that MessageID and Final fields are properly serialized
	payload := MessagePayload{
		Content:   "Test message",
		MessageID: "msg-abc-123",
		Final:     true,
	}

	msg := newMessage(TypeMessageCreate, payload)

	// Serialize to JSON
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	// Verify JSON contains message_id and final fields
	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"message_id":"msg-abc-123"`)
	assert.Contains(t, jsonStr, `"final":true`)
}

func TestKrabotChannel_PlaceholderMessageIDInPayload(t *testing.T) {
	// Test that placeholder messages have correct message_id and final=false
	payload := MessagePayload{
		Content:   "Thinking... 💭",
		MessageID: "placeholder-xyz",
		Final:     false,
	}

	msg := newMessage(TypeMessageCreate, payload)
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"message_id":"placeholder-xyz"`)
	assert.Contains(t, jsonStr, `"final":false`)
}

func TestKrabotChannel_UpdateMessageIDInPayload(t *testing.T) {
	// Test that update messages include the message_id being updated
	payload := MessagePayload{
		Content:   "Updated content",
		MessageID: "original-msg-id",
		Final:     true,
	}

	msg := newMessage(TypeMessageUpdate, payload)
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"message_id":"original-msg-id"`)
	assert.Contains(t, jsonStr, `"final":true`)
}

func TestKrabotChannel_Send_NotRunning(t *testing.T) {
	ch := setupTestChannel(t)

	ctx := context.Background()
	msg := bus.OutboundMessage{
		ChatID:  "krabot:test",
		Content: "Test",
	}

	err := ch.Send(ctx, msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}
