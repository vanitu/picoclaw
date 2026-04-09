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
	"strings"
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

	logger.DebugCF("krabot", "ActiveStorage: uploading file", map[string]any{
		"local_path":   localPath,
		"filename":     filename,
		"content_type": contentType,
		"file_size":    stat.Size(),
	})

	// 1. Request direct upload URL from Rails
	uploadURL, err := c.requestUploadURL(ctx, filename, stat.Size(), contentType)
	if err != nil {
		return nil, fmt.Errorf("request upload URL: %w", err)
	}

	logger.DebugCF("krabot", "ActiveStorage: got upload URL", map[string]any{
		"blob_id":          uploadURL.BlobID,
		"signed_id":        uploadURL.SignedID,
		"upload_url_preview": truncate(uploadURL.UploadURL, 50),
	})

	// 2. Upload file to storage service (S3, GCS, etc.)
	if err := c.uploadToStorage(ctx, uploadURL.UploadURL, uploadURL.Headers, file); err != nil {
		return nil, fmt.Errorf("upload to storage: %w", err)
	}

	// 3. Confirm upload with Rails
	result, err := c.confirmUpload(ctx, uploadURL.BlobID)
	if err != nil {
		return nil, fmt.Errorf("confirm upload: %w", err)
	}

	logger.DebugCF("krabot", "ActiveStorage: upload complete", map[string]any{
		"blob_id":   result.BlobID,
		"signed_id": result.SignedID,
	})

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

// DownloadFromURL downloads a file from a URL and saves it to a temp file.
// Returns the local file path. The caller is responsible for cleaning up the file.
func DownloadFromURL(fileURL string, maxSize int64) (string, error) {
	logger.DebugCF("krabot", "DownloadFromURL: starting download", map[string]any{
		"url_preview": truncate(fileURL, 50),
		"max_size":    maxSize,
	})

	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Get(fileURL)
	if err != nil {
		logger.DebugCF("krabot", "DownloadFromURL: request failed", map[string]any{
			"url_preview": truncate(fileURL, 50),
			"error":       err.Error(),
		})
		return "", fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()

	logger.DebugCF("krabot", "DownloadFromURL: got response", map[string]any{
		"url_preview":    truncate(fileURL, 50),
		"status_code":    resp.StatusCode,
		"content_length": resp.ContentLength,
		"content_type":   resp.Header.Get("Content-Type"),
	})

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download file: status %d", resp.StatusCode)
	}

	// Check content length if available
	if resp.ContentLength > 0 && resp.ContentLength > maxSize {
		logger.DebugCF("krabot", "DownloadFromURL: file too large", map[string]any{
			"content_length": resp.ContentLength,
			"max_size":       maxSize,
		})
		return "", fmt.Errorf("file too large: %d bytes (max %d)", resp.ContentLength, maxSize)
	}

	// Create temp file
	ext := filepath.Ext(resp.Request.URL.Path)
	if ext == "" {
		ext = ".bin"
	}

	tempFile, err := os.CreateTemp(os.TempDir(), "krabot-download-*"+ext)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tempFile.Close()

	// Copy with size limit
	limitedReader := io.LimitReader(resp.Body, maxSize+1)
	n, err := io.Copy(tempFile, limitedReader)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("write file: %w", err)
	}

	if n > maxSize {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("file too large: exceeds %d bytes", maxSize)
	}

	logger.DebugCF("krabot", "DownloadFromURL: download complete", map[string]any{
		"temp_file":    tempFile.Name(),
		"bytes_written": n,
	})

	return tempFile.Name(), nil
}

// ClassifyDownloadError classifies a download error into a DownloadError with code and recoverability.
func ClassifyDownloadError(err error) *DownloadError {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Check for specific error patterns
	switch {
	// Connection refused - not recoverable (server down/wrong address)
	case containsAny(errStr, "connection refused", "dial tcp"):
		return &DownloadError{
			Code:        "connection_refused",
			Message:     "Cannot connect to media server",
			Recoverable: false,
			Cause:       err,
		}

	// Timeout errors - potentially recoverable
	case containsAny(errStr, "timeout", "context deadline exceeded", "i/o timeout"):
		return &DownloadError{
			Code:        "timeout",
			Message:     "Media download timed out",
			Recoverable: true,
			Cause:       err,
		}

	// HTTP 4xx errors - not recoverable (client errors)
	case containsAny(errStr, "status 404", "not found"):
		return &DownloadError{
			Code:        "not_found",
			Message:     "Media file not found",
			Recoverable: false,
			Cause:       err,
		}

	case containsAny(errStr, "status 403", "forbidden"):
		return &DownloadError{
			Code:        "forbidden",
			Message:     "Access to media denied",
			Recoverable: false,
			Cause:       err,
		}

	// HTTP 5xx errors - potentially recoverable (server errors)
	case containsAny(errStr, "status 5", "internal server error", "bad gateway", "service unavailable"):
		return &DownloadError{
			Code:        "server_error",
			Message:     "Media server error",
			Recoverable: true,
			Cause:       err,
		}

	// File too large - not recoverable (client needs to upload smaller file)
	case containsAny(errStr, "file too large", "exceeds"):
		return &DownloadError{
			Code:        "file_too_large",
			Message:     "Media file exceeds size limit",
			Recoverable: false,
			Cause:       err,
		}

	// DNS resolution errors - potentially recoverable
	case containsAny(errStr, "no such host", "lookup"):
		return &DownloadError{
			Code:        "dns_error",
			Message:     "Cannot resolve media server address",
			Recoverable: true,
			Cause:       err,
		}

	// Default: unknown error, not recoverable
	default:
		return &DownloadError{
			Code:        "download_failed",
			Message:     "Failed to download media file",
			Recoverable: false,
			Cause:       err,
		}
	}
}

// containsAny checks if the string contains any of the substrings (case-insensitive).
func containsAny(s string, subs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
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

	return c.sendMediaInternal(ctx, chatID, []MediaPart{media})
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
