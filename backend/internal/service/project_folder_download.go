package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/insmtx/Leros/backend/internal/infra/db"
	"github.com/insmtx/Leros/backend/internal/projectfile"
	"github.com/insmtx/Leros/backend/internal/workspace"
	"github.com/insmtx/Leros/backend/types"
)

// DownloadProjectFolder archives one folder node and its descendants as a zip file.
func (s *projectService) DownloadProjectFolder(
	ctx context.Context,
	projectPublicID string,
	folderPublicID string,
) (io.ReadCloser, string, int64, error) {
	caller, err := requireCallerOrg(ctx)
	if err != nil {
		return nil, "", 0, err
	}
	projectPublicID = strings.TrimSpace(projectPublicID)
	folderPublicID = strings.TrimSpace(folderPublicID)
	if projectPublicID == "" || folderPublicID == "" {
		return nil, "", 0, errors.New("project_id and folder_public_id are required")
	}

	project, err := db.GetProjectByPublicID(ctx, s.db, caller.OrgID, projectPublicID)
	if err != nil {
		return nil, "", 0, err
	}
	if project == nil {
		return nil, "", 0, errors.New("project not found")
	}

	folder, err := db.GetProjectFileByProjectAndFilePublicID(ctx, s.db, caller.OrgID, project.ID, folderPublicID)
	if err != nil {
		return nil, "", 0, fmt.Errorf("get folder node: %w", err)
	}
	if folder == nil || folder.NodeType != types.ProjectFileNodeTypeFolder {
		return nil, "", 0, errors.New("folder not found")
	}
	if err := s.perm.RequireProject(ctx, FromTypeCaller(caller), project, types.ActionProjectView); err != nil {
		return nil, "", 0, err
	}

	folderPath, err := projectfile.NormalizeFolderRelativePath(folder.RelativePath)
	if err != nil {
		return nil, "", 0, err
	}

	files, err := db.ListProjectFiles(ctx, s.db, caller.OrgID, project.ID, "")
	if err != nil {
		return nil, "", 0, fmt.Errorf("list project files: %w", err)
	}
	authorized := s.perm.FilterProjectFilesByAction(ctx, FromTypeCaller(caller), files, projectFileDownloadAction)

	var entries []types.ProjectFile
	for i := range authorized {
		pf := &authorized[i]
		if pf.NodeType == types.ProjectFileNodeTypeFolder {
			continue
		}
		normalizedPath, normErr := workspace.NormalizeRelativePath(pf.RelativePath)
		if normErr != nil {
			continue
		}
		prefix := strings.TrimSuffix(folderPath, "/")
		if normalizedPath == prefix || strings.HasPrefix(normalizedPath, prefix+"/") {
			entries = append(entries, *pf)
		}
	}
	if len(entries) == 0 {
		return nil, "", 0, errors.New("folder is empty")
	}

	zipName := filepath.Base(strings.TrimSuffix(folderPath, "/")) + ".zip"
	buf := &bytes.Buffer{}
	zipWriter := zip.NewWriter(buf)
	rootName := filepath.Base(strings.TrimSuffix(folderPath, "/"))

	for _, entry := range entries {
		relativeName := strings.TrimPrefix(entry.RelativePath, strings.TrimSuffix(folderPath, "/"))
		relativeName = strings.TrimPrefix(relativeName, "/")
		zipPath := filepath.ToSlash(filepath.Join(rootName, relativeName))

		writer, zipErr := zipWriter.Create(zipPath)
		if zipErr != nil {
			return nil, "", 0, fmt.Errorf("create zip entry: %w", zipErr)
		}

		reader, _, _, openErr := openProjectFileVersion(ctx, s.db, caller.OrgID, entry.FilePublicID)
		if openErr != nil {
			return nil, "", 0, openErr
		}
		if _, copyErr := io.Copy(writer, reader); copyErr != nil {
			reader.Close()
			return nil, "", 0, fmt.Errorf("write zip entry: %w", copyErr)
		}
		reader.Close()
	}

	if err := zipWriter.Close(); err != nil {
		return nil, "", 0, fmt.Errorf("finalize zip: %w", err)
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes())), zipName, int64(buf.Len()), nil
}
