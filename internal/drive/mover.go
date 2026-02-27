package drive

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// MoveFile move um arquivo de uma pasta para outra.
func MoveFile(ctx context.Context, srv *drive.Service, fileID string, newParentID string, oldParentID string) error {
	operation := func() error {
		_, err := srv.Files.Update(fileID, nil).
			Context(ctx).
			AddParents(newParentID).
			RemoveParents(oldParentID).
			Fields("id, parents").
			Do()
		if err != nil {
			if isRetryable(err) {
				return err // retryable
			}
			return backoff.Permanent(err) // não retryable
		}
		return nil
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 2 * time.Minute
	b.InitialInterval = 1 * time.Second
	b.MaxInterval = 30 * time.Second

	if err := backoff.Retry(operation, backoff.WithContext(b, ctx)); err != nil {
		return fmt.Errorf("erro ao mover arquivo '%s': %w", fileID, err)
	}

	slog.Debug("arquivo movido", "fileID", fileID, "newParent", newParentID)
	return nil
}

// RenameFile renomeia um arquivo.
func RenameFile(ctx context.Context, srv *drive.Service, fileID string, newName string) error {
	operation := func() error {
		file := &drive.File{
			Name: newName,
		}
		_, err := srv.Files.Update(fileID, file).
			Context(ctx).
			Fields("id, name").
			Do()
		if err != nil {
			if isRetryable(err) {
				return err // retryable
			}
			return backoff.Permanent(err) // não retryable
		}
		return nil
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 2 * time.Minute
	b.InitialInterval = 1 * time.Second
	b.MaxInterval = 30 * time.Second

	if err := backoff.Retry(operation, backoff.WithContext(b, ctx)); err != nil {
		return fmt.Errorf("erro ao renomear arquivo '%s': %w", fileID, err)
	}

	slog.Debug("arquivo renomeado", "fileID", fileID, "newName", newName)
	return nil
}

// MoveAndRenameFile move e renomeia um arquivo em uma única operação.
func MoveAndRenameFile(ctx context.Context, srv *drive.Service, fileID string, newName string, newParentID string, oldParentID string) error {
	operation := func() error {
		file := &drive.File{
			Name: newName,
		}
		_, err := srv.Files.Update(fileID, file).
			Context(ctx).
			AddParents(newParentID).
			RemoveParents(oldParentID).
			Fields("id, name, parents").
			Do()
		if err != nil {
			if isRetryable(err) {
				return err // retryable
			}
			return backoff.Permanent(err) // não retryable
		}
		return nil
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 2 * time.Minute
	b.InitialInterval = 1 * time.Second
	b.MaxInterval = 30 * time.Second

	if err := backoff.Retry(operation, backoff.WithContext(b, ctx)); err != nil {
		return fmt.Errorf("erro ao mover e renomear arquivo '%s': %w", fileID, err)
	}

	slog.Debug("arquivo movido e renomeado", "fileID", fileID, "newName", newName, "newParent", newParentID)
	return nil
}

// MoveFilesToFolder move vários arquivos para uma pasta destino.
func MoveFilesToFolder(ctx context.Context, srv *drive.Service, files []*FileInfo, destFolderID string) (moved int, errors []error) {
	for _, f := range files {
		oldParent := "root"
		if len(f.Parents) > 0 {
			oldParent = f.Parents[0]
		}

		if err := MoveFile(ctx, srv, f.ID, destFolderID, oldParent); err != nil {
			errors = append(errors, fmt.Errorf("'%s': %w", f.Name, err))
			slog.Error("falha ao mover arquivo", "name", f.Name, "error", err)
			continue
		}

		moved++
	}

	return moved, errors
}

// isRetryable verifica se um erro da API Google é retryable.
func isRetryable(err error) bool {
	if apiErr, ok := err.(*googleapi.Error); ok {
		switch apiErr.Code {
		case 429, 500, 502, 503, 504:
			return true
		}
	}
	// Erros de rede
	errStr := err.Error()
	return strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout")
}
