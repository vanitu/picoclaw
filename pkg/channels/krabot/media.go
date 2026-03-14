package krabot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// ActiveStorageClient handles communication with Rails ActiveStorage.
type ActiveStorageClient struct {
	config ActiveStorageConfig
	client *http.Client
}

// NewActiveStorageClient creates a new ActiveStorage client.
func NewActiveStorageClient(cfg ActiveStorageConfig) *ActiveStorageClient {
	return &ActiveStorageClient{
		config: cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// UploadResult contains the result of an upload.
type UploadResult struct {
	BlobID    string `json:"blob_id"`
	SignedID  string `json:"signed_id"`
	DirectURL string `json:"direct_upload_url,omitempty"`
}

// UploadFile uploads a local file to ActiveStorage.
func (c *ActiveStorageClient) UploadFile(ctx context.Context, localPath string, filename string, contentType string) (*UploadResult, error) {
	if c.config.BaseURL == "" || c.config.APIKey == "" {
		return nil, fmt.Errorf("active_storage not configured")
	}

	// Open file
	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	// 1. Request direct upload URL from Rails
	uploadURL, err := c.requestUploadURL(ctx, filename, stat.Size(), contentType)
	if err != nil {
		return nil, fmt.Errorf("request upload URL: %w", err)
	}

	// 2. Upload file to storage service (S3, GCS, etc.)
	if err := c.uploadToStorage(ctx, uploadURL.UploadURL, uploadURL.Headers, file); err != nil {
		return nil, fmt.Errorf("upload to storage: %w", err)
	}

	// 3. Confirm upload with Rails
	result, err := c.confirmUpload(ctx, uploadURL.BlobID)
	if err != nil {
		return nil, fmt.Errorf("confirm upload: %w", err)
	}

	return result, nil
}

// uploadURLRequest represents a request for a direct upload URL.
type uploadURLRequest struct {
	Blob struct {
		Filename    string `json:"filename"`
		ByteSize    int64  `json:"byte_size"`
		Checksum    string `json:"checksum,omitempty"`
		ContentType string `json:"content_type"`
	} `json:"blob"`
}

// uploadURLResponse represents the response with upload URL.
type uploadURLResponse struct {
	BlobID      string            `json:"blob_id"`
	SignedID    string            `json:"signed_id"`
	UploadURL   string            `json:"upload_url"`
	Headers     map[string]string `json:"headers"`
}

// requestUploadURL gets a direct upload URL from Rails.
func (c *ActiveStorageClient) requestUploadURL(ctx context.Context, filename string, size int64, contentType string) (*uploadURLResponse, error) {
	reqBody := uploadURLRequest{}
	reqBody.Blob.Filename = filename
	reqBody.Blob.ByteSize = size
	reqBody.Blob.ContentType = contentType

	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.config.BaseURL+"/api/v1/direct_uploads",
		bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request upload URL: %s - %s", resp.Status, string(body))
	}

	var result uploadURLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// uploadToStorage uploads the file directly to the storage service.
func (c *ActiveStorageClient) uploadToStorage(ctx context.Context, uploadURL string, headers map[string]string, file io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, file)
	if err != nil {
		return err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload to storage: %s - %s", resp.Status, string(body))
	}

	return nil
}

// confirmUpload confirms the upload with Rails.
func (c *ActiveStorageClient) confirmUpload(ctx context.Context, blobID string) (*UploadResult, error) {
	// This is optional depending on Rails setup - some configurations
	// don't require explicit confirmation
	return &UploadResult{
		BlobID:   blobID,
		SignedID: blobID, // In many setups, blob_id is the signed_id
	}, nil
}

// GetSignedURL generates a signed URL for downloading a blob.
func (c *ActiveStorageClient) GetSignedURL(ctx context.Context, blobID string, expirySeconds int) (string, error) {
	if c.config.BaseURL == "" || c.config.APIKey == "" {
		return "", fmt.Errorf("active_storage not configured")
	}

	if expirySeconds <= 0 {
		expirySeconds = c.config.GetDefaultExpiry()
	}

	// Build URL to Rails API for signed URL
	u, err := url.Parse(c.config.BaseURL + "/api/v1/storage/" + blobID + "/url")
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("expires", fmt.Sprintf("%d", expirySeconds))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get signed URL: %s - %s", resp.Status, string(body))
	}

	var result struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.URL, nil
}

// DownloadFile downloads a file from a signed URL to a local temp file.
func (c *ActiveStorageClient) DownloadFile(ctx context.Context, signedURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", signedURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download file: %s", resp.Status)
	}

	// Create temp file
	tempDir := os.TempDir()
	ext := filepath.Ext(resp.Request.URL.Path)
	if ext == "" {
		ext = ".bin"
	}

	tempFile, err := os.CreateTemp(tempDir, "krabot-*"+ext)
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	// Download to temp file
	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// MediaPartFromUpload creates a MediaPart from an upload result.
func MediaPartFromUpload(result *UploadResult, mediaType, filename, contentType string) MediaPart {
	return MediaPart{
		Type:        mediaType,
		Filename:    filename,
		ContentType: contentType,
		// URL will be filled in after generating signed URL
	}
}

// SendMediaWithActiveStorage uploads AI-generated media to ActiveStorage and sends to client.
func (c *KrabotChannel) SendMediaWithActiveStorage(ctx context.Context, chatID string, localPath string, mediaType string) error {
	if c.config.ActiveStorage.BaseURL == "" {
		return fmt.Errorf("active_storage not configured")
	}

	client := NewActiveStorageClient(c.config.ActiveStorage)

	filename := filepath.Base(localPath)
	contentType := detectContentType(localPath, mediaType)

	// Upload to ActiveStorage
	uploadResult, err := client.UploadFile(ctx, localPath, filename, contentType)
	if err != nil {
		logger.ErrorCF("krabot", "Failed to upload to ActiveStorage", map[string]any{
			"error": err.Error(),
		})
		return err
	}

	// Generate signed URL
	signedURL, err := client.GetSignedURL(ctx, uploadResult.SignedID, c.config.ActiveStorage.GetDefaultExpiry())
	if err != nil {
		logger.ErrorCF("krabot", "Failed to get signed URL", map[string]any{
			"error": err.Error(),
		})
		return err
	}

	// Send to client
	media := MediaPart{
		Type:        mediaType,
		URL:         signedURL,
		Filename:    filename,
		ContentType: contentType,
	}

	return c.SendMedia(ctx, chatID, []MediaPart{media})
}

// detectContentType detects MIME type from file extension or defaults based on media type.
func detectContentType(path string, mediaType string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".ogg":
		return "audio/ogg"
	case ".pdf":
		return "application/pdf"
	}

	// Default based on media type
	switch mediaType {
	case "image":
		return "image/png"
	case "audio":
		return "audio/mpeg"
	case "video":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}
