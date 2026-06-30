package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/insmtx/Leros/backend/types"
)

func TestListTaskArtifactsDoesNotExposeDownloadURL(t *testing.T) {
	result := convertFileUploadToContractArtifact(&types.FileUpload{
		PublicID:     "file_abc123",
		Filename:     "result.md",
		OriginalName: "result.md",
		MimeType:     "text/markdown",
		FileSize:     12,
		Sha256:       "abc123",
		StorageURI:   "s3://bucket/projects/1/1/repo/result.md",
	}, time.Now())
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}
	if strings.Contains(string(payload), "download_url") {
		t.Fatalf("list response should not expose download_url: %s", payload)
	}
}
