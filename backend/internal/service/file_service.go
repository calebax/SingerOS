package service

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/ygpkg/storage-go"

	"github.com/insmtx/Leros/backend/internal/api/contract"
	infradb "github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/internal/infra/filestore"
	"github.com/ygpkg/yg-go/encryptor/snowflake"
	"github.com/ygpkg/yg-go/logs"
	"gorm.io/gorm"
)

type fileService struct {
	db *gorm.DB
}

var _ contract.FileService = (*fileService)(nil)

func NewFileService(db *gorm.DB) contract.FileService {
	return &fileService{db: db}
}

const maxUploadSize = 100 << 20 // 100MB

func (s *fileService) UploadFile(ctx context.Context, req *contract.UploadFileRequest) (*contract.UploadFileResult, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(io.LimitReader(req.File, maxUploadSize+1))
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if int64(len(data)) > maxUploadSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size of %dMB", maxUploadSize/(1<<20))
	}

	detectedMime := http.DetectContentType(data[:min(len(data), 512)])
	mimeType := req.MimeType
	if mediaType, _, err := mime.ParseMediaType(detectedMime); err == nil {
		mimeType = mediaType
	}

	ext := ""
	if idx := strings.LastIndex(req.Filename, "."); idx >= 0 {
		ext = req.Filename[idx:]
	}
	storeFilename := fmt.Sprintf("%s%s", snowflake.GenerateIDBase58(), ext)
	var key string
	if req.SourceID != "" {
		key = fmt.Sprintf("%s/%d/%s/uploads/%s", req.Purpose, caller.OrgID, req.SourceID, storeFilename)
	} else {
		key = fmt.Sprintf("%s/%d/uploads/%s", req.Purpose, caller.OrgID, storeFilename)
	}

	file, err := filestore.Upload(ctx, s.db, filestore.UploadParams{
		Data:         data,
		Filename:     storeFilename,
		OriginalName: req.Filename,
		MimeType:     mimeType,
		Size:         int64(len(data)),
		OrgID:        caller.OrgID,
		OwnerID:      caller.Uin,
		ObjectKey:    key,
		Purpose:      req.Purpose,
	})
	if err != nil {
		return nil, fmt.Errorf("upload file: %w", err)
	}

	return &contract.UploadFileResult{
		PublicID:     file.PublicID,
		Filename:     file.Filename,
		OriginalName: file.OriginalName,
		MimeType:     file.MimeType,
		FileSize:     file.FileSize,
		Sha256:       file.Sha256,
		StorageURI:   file.StorageURI,
	}, nil
}

func (s *fileService) DownloadFile(ctx context.Context, orgID uint, fileID string) (io.ReadCloser, *contract.FileDownloadInfo, error) {
	reader, fileUpload, err := filestore.OpenFileByPublicID(ctx, s.db, orgID, fileID)
	if err != nil {
		logs.ErrorContextf(ctx, "open file by public id failed: %v", err)
		return nil, nil, fmt.Errorf("get file download failed")
	}

	publicURL := ""
	fileUpload.StorageURI = strings.TrimSpace(fileUpload.StorageURI)
	if fileUpload.StorageURI != "" {
		publicURL = fileUpload.StorageURI
	}

	return reader, &contract.FileDownloadInfo{
		FileName:  fileUpload.OriginalName,
		MimeType:  fileUpload.MimeType,
		Size:      fileUpload.FileSize,
		PublicURL: publicURL,
	}, nil
}

func (s *fileService) DownloadFileByURI(ctx context.Context, orgID uint, storageURI string) (io.ReadCloser, *contract.FileDownloadInfo, error) {
	_, bucket, key, err := storage.ParseURI(strings.TrimSpace(storageURI))
	if err != nil {
		logs.ErrorContextf(ctx, "parse storage URI failed: %v", err)
		return nil, nil, fmt.Errorf("get file download failed")
	}

	obj, err := filestore.GetStorage().GetObject(ctx, bucket, key)
	if err != nil {
		logs.ErrorContextf(ctx, "get object failed: %v", err)
		return nil, nil, fmt.Errorf("get file download failed")
	}

	return obj.Body, &contract.FileDownloadInfo{
		FileName:  key[strings.LastIndex(key, "/")+1:],
		MimeType:  "",
		Size:      obj.Size,
		PublicURL: "",
	}, nil
}

func (s *fileService) PresignDownloadURL(ctx context.Context, orgID uint, publicID, storageURI string) (string, error) {
	var targetURI string

	if publicID != "" {
		fileUpload, err := infradb.GetFileUploadByPublicID(ctx, s.db, orgID, publicID)
		if err != nil {
			logs.ErrorContextf(ctx, "get file upload by publicID failed: %v", err)
			return "", fmt.Errorf("get presign download url failed")
		}
		if fileUpload == nil {
			return "", fmt.Errorf("get presign download url failed")
		}
		targetURI = fileUpload.StorageURI
	} else {
		targetURI = storageURI
	}

	if targetURI == "" {
		return "", fmt.Errorf("publicID or storageURI is required")
	}

	_, bucket, key, err := storage.ParseURI(targetURI)
	if err != nil {
		logs.ErrorContextf(ctx, "parse storage URI failed: %v", err)
		return "", fmt.Errorf("get presign download url failed")
	}

	url, _, err := filestore.PresignDownload(ctx, bucket, key)
	if err != nil {
		logs.ErrorContextf(ctx, "presign download failed: %v", err)
		return "", fmt.Errorf("get presign download url failed")
	}
	return url, nil
}
