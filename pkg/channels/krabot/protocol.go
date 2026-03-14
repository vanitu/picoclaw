package krabot

import "time"

// Message types for Krabot protocol.
const (
	// Client → Server
	TypeMessageSend  = "message.send"
	TypeMediaSend    = "media.send"
	TypePing         = "ping"

	// Server → Client
	TypeMessageCreate = "message.create"
	TypeMessageUpdate = "message.update"
	TypeMediaCreate   = "media.create"
	TypeTypingStart   = "typing.start"
	TypeTypingStop    = "typing.stop"
	TypePong          = "pong"
	TypeError         = "error"
)

// KrabotMessage is the wire format for all Krabot Protocol messages.
type KrabotMessage struct {
	Type      string         `json:"type"`
	ID        string         `json:"id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Timestamp int64          `json:"timestamp,omitempty"`
	Payload   MessagePayload `json:"payload,omitempty"`
}

// MessagePayload contains the message content and media.
type MessagePayload struct {
	Content string      `json:"content,omitempty"`
	Media   []MediaPart `json:"media,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
}

// MediaPart represents a media attachment with ActiveStorage signed URL.
type MediaPart struct {
	Type        string `json:"type"`                   // "image", "audio", "video", "file"
	URL         string `json:"url"`                    // ActiveStorage signed URL
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size,omitempty"`
	Caption     string `json:"caption,omitempty"`
}

// ErrorInfo contains error details.
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// newMessage creates a KrabotMessage with the given type and payload.
func newMessage(msgType string, payload MessagePayload) KrabotMessage {
	return KrabotMessage{
		Type:      msgType,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	}
}

// newError creates an error KrabotMessage.
func newError(code, message string) KrabotMessage {
	return newMessage(TypeError, MessagePayload{
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	})
}

// newTextMessage creates a text message response.
func newTextMessage(content string) KrabotMessage {
	return newMessage(TypeMessageCreate, MessagePayload{
		Content: content,
	})
}

// newMediaMessage creates a media message response.
func newMediaMessage(media []MediaPart) KrabotMessage {
	return newMessage(TypeMediaCreate, MessagePayload{
		Media: media,
	})
}
