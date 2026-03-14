package krabot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMessage(t *testing.T) {
	msg := newMessage(TypeMessageCreate, MessagePayload{
		Content: "Hello, world!",
	})

	assert.Equal(t, TypeMessageCreate, msg.Type)
	assert.Equal(t, "Hello, world!", msg.Payload.Content)
	assert.NotZero(t, msg.Timestamp)
}

func TestNewError(t *testing.T) {
	msg := newError("invalid_request", "Missing required field")

	assert.Equal(t, TypeError, msg.Type)
	assert.NotNil(t, msg.Payload.Error)
	assert.Equal(t, "invalid_request", msg.Payload.Error.Code)
	assert.Equal(t, "Missing required field", msg.Payload.Error.Message)
	assert.NotZero(t, msg.Timestamp)
}

func TestNewTextMessage(t *testing.T) {
	content := "This is a test message"
	msg := newTextMessage(content)

	assert.Equal(t, TypeMessageCreate, msg.Type)
	assert.Equal(t, content, msg.Payload.Content)
	assert.Nil(t, msg.Payload.Error)
	assert.Empty(t, msg.Payload.Media)
	assert.NotZero(t, msg.Timestamp)
}

func TestNewMediaMessage(t *testing.T) {
	media := []MediaPart{
		{
			Type:        "image",
			URL:         "https://example.com/image.jpg",
			Filename:    "image.jpg",
			ContentType: "image/jpeg",
		},
		{
			Type:        "file",
			URL:         "https://example.com/doc.pdf",
			Filename:    "doc.pdf",
			ContentType: "application/pdf",
		},
	}

	msg := newMediaMessage(media)

	assert.Equal(t, TypeMediaCreate, msg.Type)
	assert.Len(t, msg.Payload.Media, 2)
	assert.Equal(t, "image", msg.Payload.Media[0].Type)
	assert.Equal(t, "https://example.com/image.jpg", msg.Payload.Media[0].URL)
	assert.Equal(t, "file", msg.Payload.Media[1].Type)
	assert.NotZero(t, msg.Timestamp)
}

func TestMessageTypes(t *testing.T) {
	// Test all message type constants
	tests := []struct {
		name     string
		msgType  string
		expected string
	}{
		{"MessageSend", TypeMessageSend, "message.send"},
		{"Ping", TypePing, "ping"},
		{"MessageCreate", TypeMessageCreate, "message.create"},
		{"MessageUpdate", TypeMessageUpdate, "message.update"},
		{"MediaCreate", TypeMediaCreate, "media.create"},
		{"TypingStart", TypeTypingStart, "typing.start"},
		{"TypingStop", TypeTypingStop, "typing.stop"},
		{"Pong", TypePong, "pong"},
		{"Error", TypeError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.msgType)
		})
	}
}

func TestMediaPart(t *testing.T) {
	media := MediaPart{
		Type:        "video",
		URL:         "https://storage.example.com/video.mp4?signature=abc",
		Filename:    "video.mp4",
		ContentType: "video/mp4",
		Size:        1024000,
		Caption:     "A sample video",
	}

	assert.Equal(t, "video", media.Type)
	assert.Equal(t, "https://storage.example.com/video.mp4?signature=abc", media.URL)
	assert.Equal(t, "video.mp4", media.Filename)
	assert.Equal(t, "video/mp4", media.ContentType)
	assert.Equal(t, int64(1024000), media.Size)
	assert.Equal(t, "A sample video", media.Caption)
}

func TestErrorInfo(t *testing.T) {
	errInfo := ErrorInfo{
		Code:    "auth_failed",
		Message: "Invalid authentication token",
	}

	assert.Equal(t, "auth_failed", errInfo.Code)
	assert.Equal(t, "Invalid authentication token", errInfo.Message)
}
