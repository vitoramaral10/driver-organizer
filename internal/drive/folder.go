package drive

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"google.golang.org/api/drive/v3"
)

// FindFolderByName procura uma pasta pelo nome dentro de um parent.
func FindFolderByName(ctx context.Context, srv *drive.Service, name string, parentID string) (*FileInfo, error) {
	query := fmt.Sprintf("name = '%s' and '%s' in parents and mimeType = 'application/vnd.google-apps.folder' and trashed = false",
		escapeDriveQuery(name), parentID)

	result, err := srv.Files.List().
		Context(ctx).
		Q(query).
		PageSize(1).
		Fields("files(id, name, mimeType, parents)").
		Do()
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar pasta '%s': %w", name, err)
	}

	if len(result.Files) == 0 {
		return nil, nil
	}

	f := result.Files[0]
	return &FileInfo{
		ID:       f.Id,
		Name:     f.Name,
		MimeType: f.MimeType,
		Parents:  f.Parents,
	}, nil
}

// CreateFolder cria uma pasta no Drive.
func CreateFolder(ctx context.Context, srv *drive.Service, name string, parentID string) (*FileInfo, error) {
	folder := &drive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
		Parents:  []string{parentID},
	}

	created, err := srv.Files.Create(folder).
		Context(ctx).
		Fields("id, name, mimeType, parents").
		Do()
	if err != nil {
		return nil, fmt.Errorf("erro ao criar pasta '%s': %w", name, err)
	}

	slog.Info("pasta criada", "name", name, "id", created.Id)
	return &FileInfo{
		ID:       created.Id,
		Name:     created.Name,
		MimeType: created.MimeType,
		Parents:  created.Parents,
	}, nil
}

// FindOrCreateFolder busca uma pasta pelo nome ou cria se não existir.
func FindOrCreateFolder(ctx context.Context, srv *drive.Service, name string, parentID string) (*FileInfo, error) {
	existing, err := FindFolderByName(ctx, srv, name, parentID)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		slog.Debug("pasta já existe", "name", name, "id", existing.ID)
		return existing, nil
	}

	return CreateFolder(ctx, srv, name, parentID)
}

// FindOrCreateNestedFolder cria (ou encontra) pastas aninhadas, ex: "backup".
func FindOrCreateNestedFolder(ctx context.Context, srv *drive.Service, path string, rootParentID string) (*FileInfo, error) {
	parts := strings.Split(path, "/")
	currentParent := rootParentID
	var lastFolder *FileInfo

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		folder, err := FindOrCreateFolder(ctx, srv, part, currentParent)
		if err != nil {
			return nil, fmt.Errorf("erro ao criar/encontrar pasta '%s': %w", part, err)
		}

		currentParent = folder.ID
		lastFolder = folder
	}

	return lastFolder, nil
}

// ListFolders lista todas as pastas dentro de um parent.
func ListFolders(ctx context.Context, srv *drive.Service, parentID string) ([]*FileInfo, error) {
	var folders []*FileInfo
	pageToken := ""

	query := fmt.Sprintf("'%s' in parents and mimeType = 'application/vnd.google-apps.folder' and trashed = false", parentID)

	for {
		req := srv.Files.List().
			Context(ctx).
			Q(query).
			PageSize(1000).
			Fields("nextPageToken, files(id, name, mimeType, parents)").
			OrderBy("name")

		if pageToken != "" {
			req = req.PageToken(pageToken)
		}

		result, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("erro ao listar pastas: %w", err)
		}

		for _, f := range result.Files {
			folders = append(folders, &FileInfo{
				ID:       f.Id,
				Name:     f.Name,
				MimeType: f.MimeType,
				Parents:  f.Parents,
			})
		}

		pageToken = result.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return folders, nil
}

func escapeDriveQuery(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return s
}
