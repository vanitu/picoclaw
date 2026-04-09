package krabot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		mediaType string
		want      string
	}{
		// Image formats
		{
			name:      "JPEG by extension",
			path:      "/tmp/image.jpg",
			mediaType: "image",
			want:      "image/jpeg",
		},
		{
			name:      "JPEG alternate extension",
			path:      "/tmp/image.jpeg",
			mediaType: "image",
			want:      "image/jpeg",
		},
		{
			name:      "PNG by extension",
			path:      "/tmp/image.png",
			mediaType: "image",
			want:      "image/png",
		},
		{
			name:      "GIF by extension",
			path:      "/tmp/animation.gif",
			mediaType: "image",
			want:      "image/gif",
		},
		// Video formats
		{
			name:      "MP4 by extension",
			path:      "/tmp/video.mp4",
			mediaType: "video",
			want:      "video/mp4",
		},
		// Audio formats
		{
			name:      "MP3 by extension",
			path:      "/tmp/audio.mp3",
			mediaType: "audio",
			want:      "audio/mpeg",
		},
		{
			name:      "OGG by extension",
			path:      "/tmp/audio.ogg",
			mediaType: "audio",
			want:      "audio/ogg",
		},
		// Document formats
		{
			name:      "PDF by extension",
			path:      "/tmp/document.pdf",
			mediaType: "file",
			want:      "application/pdf",
		},
		// Default by media type when no extension
		{
			name:      "image default",
			path:      "/tmp/image",
			mediaType: "image",
			want:      "image/png",
		},
		{
			name:      "audio default",
			path:      "/tmp/audio",
			mediaType: "audio",
			want:      "audio/mpeg",
		},
		{
			name:      "video default",
			path:      "/tmp/video",
			mediaType: "video",
			want:      "video/mp4",
		},
		{
			name:      "unknown default",
			path:      "/tmp/unknown",
			mediaType: "unknown",
			want:      "application/octet-stream",
		},
		// Unknown extension
		{
			name:      "unknown extension uses media type default",
			path:      "/tmp/file.xyz",
			mediaType: "image",
			want:      "image/png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectContentType(tt.path, tt.mediaType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMediaPartFromUpload(t *testing.T) {
	result := &UploadResult{
		BlobID:   "blob-123",
		SignedID: "signed-456",
	}

	media := MediaPartFromUpload(result, "image", "photo.jpg", "image/jpeg")

	assert.Equal(t, "image", media.Type)
	assert.Equal(t, "photo.jpg", media.Filename)
	assert.Equal(t, "image/jpeg", media.ContentType)
	assert.Empty(t, media.URL) // URL is filled in later
}

func TestActiveStorageClient_NotConfigured(t *testing.T) {
	// Client with empty config
	client := NewActiveStorageClient(ActiveStorageConfig{})
	assert.NotNil(t, client)

	// GetSignedURL should fail with not configured error
	_, err := client.GetSignedURL(nil, "blob-123", 3600)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")

	// UploadFile should fail with not configured error
	_, err = client.UploadFile(nil, "/tmp/test.txt", "test.txt", "text/plain")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestActiveStorageClient_Configured(t *testing.T) {
	cfg := ActiveStorageConfig{
		BaseURL:       "https://storage.example.com",
		APIKey:        "test-api-key",
		DefaultExpiry: 3600,
	}

	client := NewActiveStorageClient(cfg)
	assert.NotNil(t, client)
	assert.Equal(t, cfg.BaseURL, client.config.BaseURL)
	assert.Equal(t, cfg.APIKey, client.config.APIKey)
}
