package ssh

import (
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
	Mode    string `json:"mode"`
}

func sftpNewClient(client *ssh.Client) (*sftp.Client, error) {
	return sftp.NewClient(client)
}

func (m *Manager) SFTPListDir(ctx context.Context, nodeID, dirPath string) ([]FileEntry, error) {
	client, err := m.getClient(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	defer m.releaseClient(nodeID, client)

	if dirPath == "" {
		dirPath = "/"
	}

	sftp, err := sftpNewClient(client)
	if err != nil {
		return nil, fmt.Errorf("sftp client: %w", err)
	}
	defer sftp.Close()

	entries, err := sftp.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dirPath, err)
	}

	var files []FileEntry
	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		fullPath := path.Join(dirPath, name)
		files = append(files, FileEntry{
			Name:    name,
			Path:    fullPath,
			IsDir:   e.IsDir(),
			Size:    e.Size(),
			ModTime: e.ModTime().Unix(),
			Mode:    e.Mode().String(),
		})
	}

	sort.SliceStable(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	return files, nil
}

func (m *Manager) SFTPUpload(ctx context.Context, nodeID, remotePath string, content io.Reader) error {
	client, err := m.getClient(ctx, nodeID)
	if err != nil {
		return err
	}
	defer m.releaseClient(nodeID, client)

	sftp, err := sftpNewClient(client)
	if err != nil {
		return fmt.Errorf("sftp client: %w", err)
	}
	defer sftp.Close()

	dir := path.Dir(remotePath)
	if err := sftp.MkdirAll(dir); err != nil {
	}

	f, err := sftp.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote file: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, content)
	return err
}

func (m *Manager) SFTPDownload(ctx context.Context, nodeID, remotePath string) (io.ReadCloser, error) {
	client, err := m.getClient(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	defer m.releaseClient(nodeID, client)

	sftp, err := sftpNewClient(client)
	if err != nil {
		return nil, fmt.Errorf("sftp client: %w", err)
	}

	f, err := sftp.Open(remotePath)
	if err != nil {
		sftp.Close()
		return nil, fmt.Errorf("open remote file: %w", err)
	}

	return &sftpReadCloser{file: f, sftp: sftp}, nil
}

type sftpReadCloser struct {
	file interface {
		io.Reader
		io.Closer
	}
	sftp interface {
		io.Closer
	}
}

func (s *sftpReadCloser) Read(p []byte) (int, error) { return s.file.Read(p) }
func (s *sftpReadCloser) Close() error {
	s.file.Close()
	return s.sftp.Close()
}

func (m *Manager) SFTPDelete(ctx context.Context, nodeID, remotePath string) error {
	client, err := m.getClient(ctx, nodeID)
	if err != nil {
		return err
	}
	defer m.releaseClient(nodeID, client)

	sftp, err := sftpNewClient(client)
	if err != nil {
		return err
	}
	defer sftp.Close()

	info, err := sftp.Stat(remotePath)
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}

	if info.IsDir() {
		return sftp.RemoveAll(remotePath)
	}
	return sftp.Remove(remotePath)
}

func (m *Manager) SFTPMkdir(ctx context.Context, nodeID, dirPath string) error {
	client, err := m.getClient(ctx, nodeID)
	if err != nil {
		return err
	}
	defer m.releaseClient(nodeID, client)

	sftp, err := sftpNewClient(client)
	if err != nil {
		return err
	}
	defer sftp.Close()

	return sftp.MkdirAll(dirPath)
}

func (m *Manager) SFTPRename(ctx context.Context, nodeID, oldPath, newPath string) error {
	client, err := m.getClient(ctx, nodeID)
	if err != nil {
		return err
	}
	defer m.releaseClient(nodeID, client)

	sftp, err := sftpNewClient(client)
	if err != nil {
		return err
	}
	defer sftp.Close()

	return sftp.Rename(oldPath, newPath)
}
