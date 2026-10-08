package sandbox

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
)

const MaxDownloadBytes = 50 * 1024 * 1024

// WorkspacePath rejects private browser state and paths outside user files.
func WorkspacePath(value string) (string, error) {
	value = strings.TrimPrefix(value, "/workspace/")
	if value == "" || strings.HasPrefix(value, "/") || strings.ContainsRune(value, 0) {
		return "", fmt.Errorf("invalid workspace path")
	}
	for _, part := range strings.Split(value, "/") {
		if strings.HasPrefix(part, ".") {
			return "", fmt.Errorf("private or relative paths are not downloadable")
		}
	}
	return path.Join("/workspace", value), nil
}

// CopyWorkspaceFile streams a verified regular file, without buffering binaries in tool history.
func (m *Manager) CopyWorkspaceFile(ctx context.Context, id, name string, send func(io.Reader, int64) error) error {
	stream, _, err := m.docker.CopyFromContainer(ctx, id, name)
	if err != nil {
		return err
	}
	defer stream.Close()
	archive := tar.NewReader(stream)
	header, err := archive.Next()
	if err != nil {
		return err
	}
	if header.Typeflag != tar.TypeReg || header.Size > MaxDownloadBytes {
		return fmt.Errorf("file is not regular or exceeds 50 MB")
	}
	return send(io.LimitReader(archive, header.Size), header.Size)
}
