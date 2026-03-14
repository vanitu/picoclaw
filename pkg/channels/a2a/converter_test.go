package a2a

import (
	"testing"

	"github.com/sipeed/picoclaw/pkg/media"
	"github.com/stretchr/testify/assert"
)

func TestConverter_PartsToInbound_TextOnly(t *testing.T) {
	channel := &A2AChannel{
		config: A2AConfig{},
	}
	converter := NewConverter(channel)

	parts := []Part{
		{Type: PartTypeText, Text: "Hello, world!"},
	}

	inbound, err := converter.PartsToInbound("task-123", "session-456", parts)

	assert.NoError(t, err)
	assert.Equal(t, "a2a:task-123", inbound.ChatID)
	assert.Equal(t, "Hello, world!", inbound.Content)
	assert.Empty(t, inbound.Media)
	assert.Equal(t, "a2a", inbound.Sender.Platform)
	assert.Equal(t, "task-123", inbound.Metadata["task_id"])
	assert.Equal(t, "session-456", inbound.Metadata["session_id"])
}

func TestConverter_PartsToInbound_MultipleTextParts(t *testing.T) {
	channel := &A2AChannel{
		config: A2AConfig{},
	}
	converter := NewConverter(channel)

	parts := []Part{
		{Type: PartTypeText, Text: "First line"},
		{Type: PartTypeText, Text: "Second line"},
	}

	inbound, err := converter.PartsToInbound("task-123", "", parts)

	assert.NoError(t, err)
	assert.Equal(t, "First line\nSecond line", inbound.Content)
}

func TestConverter_PartsToInbound_DataPart(t *testing.T) {
	channel := &A2AChannel{
		config: A2AConfig{},
	}
	converter := NewConverter(channel)

	parts := []Part{
		{Type: PartTypeText, Text: "Here is some data:"},
		{Type: PartTypeData, Data: map[string]interface{}{"key": "value", "num": 42}},
	}

	inbound, err := converter.PartsToInbound("task-123", "", parts)

	assert.NoError(t, err)
	assert.Contains(t, inbound.Content, "Here is some data:")
	assert.Contains(t, inbound.Content, "[data:")
	assert.Contains(t, inbound.Content, "key")
	assert.Contains(t, inbound.Content, "value")
}

func TestConverter_PartsToInbound_EmptyParts(t *testing.T) {
	channel := &A2AChannel{
		config: A2AConfig{},
	}
	converter := NewConverter(channel)

	inbound, err := converter.PartsToInbound("task-123", "", []Part{})

	assert.NoError(t, err)
	assert.Equal(t, "", inbound.Content)
	assert.Empty(t, inbound.Media)
}

func TestConverter_sanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"file.txt", "file.txt"},
		{"/path/to/file.txt", "file.txt"},
		{"\\windows\\path\\file.txt", "file.txt"},
		{"../../../etc/passwd", "passwd"},
		{"normal-file.jpg", "normal-file.jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestActiveStorageConfig_IsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		config   ActiveStorageConfig
		expected bool
	}{
		{
			name:     "empty config",
			config:   ActiveStorageConfig{},
			expected: false,
		},
		{
			name: "only base_url",
			config: ActiveStorageConfig{
				BaseURL: "https://example.com",
			},
			expected: false,
		},
		{
			name: "only api_key",
			config: ActiveStorageConfig{
				APIKey: "secret",
			},
			expected: false,
		},
		{
			name: "both configured",
			config: ActiveStorageConfig{
				BaseURL: "https://example.com",
				APIKey:  "secret",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsConfigured()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConverter_ArtifactsToParts_NotConfigured(t *testing.T) {
	channel := &A2AChannel{
		config: A2AConfig{
			ActiveStorage: ActiveStorageConfig{},
		},
	}
	converter := NewConverter(channel)

	_, err := converter.ArtifactsToParts([]string{"media://test"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

// Mock implementations for testing

type mockMediaStore struct {
	resolveFunc func(ref string) (string, error)
}

func (m *mockMediaStore) Store(localPath string, meta media.MediaMeta, scope string) (string, error) {
	return "media://mock", nil
}

func (m *mockMediaStore) Resolve(ref string) (string, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(ref)
	}
	return "/tmp/mock", nil
}

func (m *mockMediaStore) ResolveWithMeta(ref string) (string, media.MediaMeta, error) {
	return "/tmp/mock", media.MediaMeta{Filename: "mock.txt"}, nil
}

func (m *mockMediaStore) ReleaseAll(scope string) error {
	return nil
}
