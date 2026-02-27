package drive

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cenkalti/backoff/v4"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// FileInfo contém metadados de um arquivo do Drive.
type FileInfo struct {
	ID           string
	Name         string
	MimeType     string
	Parents      []string
	CreatedTime  string
	ModifiedTime string
	Size         int64
}

// IsFolder retorna true se o arquivo é uma pasta.
func (f *FileInfo) IsFolder() bool {
	return f.MimeType == "application/vnd.google-apps.folder"
}

// ListAllFiles lista todos os arquivos na raiz do "Meu Drive".
func ListAllFiles(ctx context.Context, srv *drive.Service) ([]*FileInfo, error) {
	return listFilesInFolder(ctx, srv, "root")
}

// ListFilesInFolder lista todos os arquivos em uma pasta específica.
func ListFilesInFolder(ctx context.Context, srv *drive.Service, folderID string) ([]*FileInfo, error) {
	return listFilesInFolder(ctx, srv, folderID)
}

func listFilesInFolder(ctx context.Context, srv *drive.Service, folderID string) ([]*FileInfo, error) {
	var allFiles []*FileInfo
	pageToken := ""
	query := fmt.Sprintf("'%s' in parents and trashed = false", folderID)

	for {
		var result *drive.FileList
		
		// Usar backoff exponencial para lidar com rate limiting e erros 500
		operation := func() error {
			req := srv.Files.List().
				Context(ctx).
				Q(query).
				PageSize(100). // Reduzido de 1000 para 100 para evitar timeouts
				Fields("nextPageToken, files(id, name, mimeType, parents, createdTime, modifiedTime, size)").
				OrderBy("name")

			if pageToken != "" {
				req = req.PageToken(pageToken)
			}

			var err error
			result, err = req.Do()
			if err != nil {
				// Verificar se é erro recuperável
				if apiErr, ok := err.(*googleapi.Error); ok {
					if apiErr.Code == 500 || apiErr.Code == 503 || apiErr.Code == 429 {
						slog.Warn("erro temporário ao listar arquivos, tentando novamente", "code", apiErr.Code, "folder", folderID)
						return err // Retry
					}
				}
				return backoff.Permanent(fmt.Errorf("erro ao listar arquivos: %w", err))
			}
			return nil
		}

		// Configurar backoff
		expBackoff := backoff.NewExponentialBackOff()
		expBackoff.InitialInterval = 1 * time.Second
		expBackoff.MaxInterval = 30 * time.Second
		expBackoff.MaxElapsedTime = 2 * time.Minute

		if err := backoff.Retry(operation, backoff.WithContext(expBackoff, ctx)); err != nil {
			return nil, err
		}

		for _, f := range result.Files {
			allFiles = append(allFiles, &FileInfo{
				ID:           f.Id,
				Name:         f.Name,
				MimeType:     f.MimeType,
				Parents:      f.Parents,
				CreatedTime:  f.CreatedTime,
				ModifiedTime: f.ModifiedTime,
				Size:         f.Size,
			})
		}

		slog.Debug("arquivos listados", "count", len(result.Files), "total", len(allFiles), "folder", folderID)

		pageToken = result.NextPageToken
		if pageToken == "" {
			break
		}
		
		// Pequeno delay entre páginas para evitar rate limiting
		time.Sleep(100 * time.Millisecond)
	}

	slog.Info("total de arquivos encontrados", "count", len(allFiles), "folder", folderID)
	return allFiles, nil
}

// ListAllFilesRecursive lista todos os arquivos recursivamente a partir de uma pasta.
func ListAllFilesRecursive(ctx context.Context, srv *drive.Service, folderID string) ([]*FileInfo, error) {
	return listAllFilesRecursiveWithDepth(ctx, srv, folderID, 0)
}

func listAllFilesRecursiveWithDepth(ctx context.Context, srv *drive.Service, folderID string, depth int) ([]*FileInfo, error) {
	if depth > 20 {
		slog.Warn("profundidade máxima de recursão atingida", "depth", depth, "folder", folderID)
		return nil, fmt.Errorf("profundidade máxima de pastas excedida (20 níveis)")
	}
	
	files, err := listFilesInFolder(ctx, srv, folderID)
	if err != nil {
		return nil, err
	}

	var allFiles []*FileInfo
	for _, f := range files {
		if f.IsFolder() {
			slog.Debug("entrando em subpasta", "folder", f.Name, "depth", depth)
			
			// Delay entre chamadas recursivas para evitar rate limiting
			time.Sleep(200 * time.Millisecond)
			
			subFiles, err := listAllFilesRecursiveWithDepth(ctx, srv, f.ID, depth+1)
			if err != nil {
				slog.Error("erro ao listar pasta, continuando", "folder", f.Name, "error", err)
				// Continuar mesmo com erro em subpasta
				continue
			}
			allFiles = append(allFiles, subFiles...)
		} else {
			allFiles = append(allFiles, f)
		}
	}

	return allFiles, nil
}
