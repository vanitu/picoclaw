package a2a

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
)

// ActiveStorageClient handles communication with Rails ActiveStorage.
type ActiveStorageClient struct {
	config ActiveStorageConfig
	client *http.Client
}

// UploadResult contains the result of an upload.
type UploadResult struct {
	BlobID   string `json:"blob_id"`
	SignedID string `json:"signed_id"`
}

// NewActiveStorageClient creates a new ActiveStorage client.
func NewActiveStorageClient(cfg ActiveStorageConfig) *ActiveStorageClient {
	return &ActiveStorageClient{
		config: cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// UploadFile uploads a local file to ActiveStorage and returns the signed ID.
func (c *ActiveStorageClient) UploadFile(ctx context.Context, localPath, filename, contentType string) (*UploadResult, error) {
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
	uploadURLResp, err := c.requestUploadURL(ctx, filename, stat.Size(), contentType)
	if err != nil {
		return nil, fmt.Errorf("request upload URL: %w", err)
	}

	// 2. Upload file to storage service (S3, GCS, etc.)
	if err := c.uploadToStorage(ctx, uploadURLResp.UploadURL, uploadURLResp.Headers, file); err != nil {
		return nil, fmt.Errorf("upload to storage: %w", err)
	}

	return &UploadResult{
		BlobID:   uploadURLResp.BlobID,
		SignedID: uploadURLResp.SignedID,
	}, nil
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
	BlobID    string            `json:"blob_id"`
	SignedID  string            `json:"signed_id"`
	UploadURL string            `json:"upload_url"`
	Headers   map[string]string `json:"headers"`
}

// requestUploadURL gets a direct upload URL from Rails.
func (c *ActiveStorageClient) requestUploadURL(ctx context.Context, filename string, size int64, contentType string) (*uploadURLResponse, error) {
	reqBody := uploadURLRequest{}
	reqBody.Blob.Filename = filename
	reqBody.Blob.ByteSize = size
	reqBody.Blob.ContentType = contentType

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

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

// GetSignedURL generates a signed URL for downloading a blob.
func (c *ActiveStorageClient) GetSignedURL(ctx context.Context, blobID string, expirySeconds int) (string, error) {
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

	// Create temp file with appropriate extension
	tempDir := os.TempDir()
	ext := filepath.Ext(resp.Request.URL.Path)
	if ext == "" {
		ext = ".bin"
	}

	tempFile, err := os.CreateTemp(tempDir, "a2a-*"+ext)
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
