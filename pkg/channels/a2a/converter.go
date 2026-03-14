package a2a

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/media"
)

// Converter handles conversion between A2A protocol and internal message formats.
type Converter struct {
	channel *A2AChannel
}

// NewConverter creates a new converter for the given channel.
func NewConverter(channel *A2AChannel) *Converter {
	return &Converter{channel: channel}
}

// PartsToInbound converts A2A message parts to a bus.InboundMessage.
func (c *Converter) PartsToInbound(taskID, sessionID string, parts []Part) (*bus.InboundMessage, error) {
	var content strings.Builder
	var fileRefs []string

	// Build media scope for this task
	scope := channels.BuildMediaScope("a2a", taskID, "")

	for _, part := range parts {
		switch part.Type {
		case PartTypeText:
			if content.Len() > 0 {
				content.WriteString("\n")
			}
			content.WriteString(part.Text)

		case PartTypeFile:
			if part.File == nil {
				continue
			}

			localPath, err := c.handleFilePart(part.File)
			if err != nil {
				// Log error but continue processing other parts
				continue
			}

			// Store in MediaStore to get a reference
			store := c.channel.GetMediaStore()
			if store != nil {
				ref, err := store.Store(localPath, media.MediaMeta{
					Filename:    part.File.Name,
					ContentType: part.File.MimeType,
					Source:      "a2a",
				}, scope)
				if err == nil {
					fileRefs = append(fileRefs, ref)
				}
			}

			if content.Len() > 0 {
				content.WriteString("\n")
			}
			content.WriteString(fmt.Sprintf("[file: %s]", part.File.Name))

		case PartTypeData:
			// Include structured data as JSON in content
			if part.Data != nil {
				jsonData, err := json.Marshal(part.Data)
				if err == nil {
					if content.Len() > 0 {
						content.WriteString("\n")
					}
					content.WriteString(fmt.Sprintf("[data: %s]", string(jsonData)))
				}
			}
		}
	}

	return &bus.InboundMessage{
		ChatID:  fmt.Sprintf("a2a:%s", taskID),
		Content: content.String(),
		Media:   fileRefs,
		Sender: bus.SenderInfo{
			Platform:    "a2a",
			PlatformID:  taskID,
			CanonicalID: fmt.Sprintf("a2a:%s", taskID),
		},
		Metadata: map[string]string{
			"task_id":    taskID,
			"session_id": sessionID,
		},
	}, nil
}

// handleFilePart handles a file part, downloading from URI or decoding base64.
func (c *Converter) handleFilePart(file *FilePart) (string, error) {
	if file.URI != "" {
		return c.downloadFile(file.URI, file.Name)
	}
	if file.Bytes != "" {
		return c.decodeBase64File(file.Bytes, file.Name)
	}
	return "", fmt.Errorf("file has no URI or bytes")
}

// downloadFile downloads a file from a URL to a temp file.
func (c *Converter) downloadFile(uri, filename string) (string, error) {
	resp, err := http.Get(uri)
	if err != nil {
		return "", fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download file: status %d", resp.StatusCode)
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "a2a-*-"+sanitizeFilename(filename))
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("write file: %w", err)
	}

	return tmpFile.Name(), nil
}

// decodeBase64File decodes base64 data to a temp file.
func (c *Converter) decodeBase64File(data, filename string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "a2a-*-"+sanitizeFilename(filename))
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	_, err = tmpFile.Write(decoded)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("write file: %w", err)
	}

	return tmpFile.Name(), nil
}

// sanitizeFilename removes path components and sanitizes the filename.
func sanitizeFilename(filename string) string {
	// Remove any path components
	parts := strings.Split(filename, "/")
	if len(parts) > 0 {
		filename = parts[len(parts)-1]
	}
	parts = strings.Split(filename, "\\")
	if len(parts) > 0 {
		filename = parts[len(parts)-1]
	}
	return filename
}

// ArtifactsToParts converts agent output files to A2A artifact parts.
// Requires ActiveStorage to be configured for uploading files.
func (c *Converter) ArtifactsToParts(fileRefs []string) ([]Part, error) {
	if !c.channel.config.ActiveStorage.IsConfigured() {
		return nil, fmt.Errorf("active_storage not configured")
	}

	client := NewActiveStorageClient(c.channel.config.ActiveStorage)
	var parts []Part

	for _, ref := range fileRefs {
		// Resolve ref to local path
		store := c.channel.GetMediaStore()
		if store == nil {
			continue
		}

		localPath, meta, err := store.ResolveWithMeta(ref)
		if err != nil {
			continue
		}

		// Upload to ActiveStorage
		result, err := client.UploadFile(context.Background(), localPath, meta.Filename, meta.ContentType)
		if err != nil {
			continue
		}

		// Get signed URL for download
		signedURL, err := client.GetSignedURL(context.Background(), result.SignedID, c.channel.config.ActiveStorage.GetDefaultExpiry())
		if err != nil {
			continue
		}

		parts = append(parts, Part{
			Type: PartTypeFile,
			File: &FilePart{
				Name:     meta.Filename,
				MimeType: meta.ContentType,
				URI:      signedURL,
			},
		})
	}

	return parts, nil
}
