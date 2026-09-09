package remote

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gamevault/pkg/config"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// RemoteClient is the common interface for SFTP / FTP access
type RemoteClient interface {
	Connect(cfg config.ServerConfig) error
	Close() error
	ScanRepository(rootPath string) ([]RemoteItem, error)
	ListDirectory(dirPath string) ([]RemoteItem, error)
	OpenRead(remoteFilePath string, offset int64) (io.ReadCloser, int64, error)
	GetFileSize(remoteFilePath string) (int64, error)
	ListFilesRecursive(dirPath string) ([]RemoteFileEntry, error)
}

// RemoteFileEntry represents a file in a remote directory tree
type RemoteFileEntry struct {
	RelativePath string `json:"relativePath"`
	FullPath     string `json:"fullPath"`
	SizeBytes    int64  `json:"sizeBytes"`
	ModTime      int64  `json:"modTime"`
}

// ClientFactory creates the appropriate client based on protocol
func NewRemoteClient(protocol string) RemoteClient {
	if strings.ToLower(protocol) == "ftp" {
		return &FTPClient{}
	}
	return &SFTPClient{}
}

// ==========================================
// SFTP Client Implementation
// ==========================================

type SFTPClient struct {
	mu         sync.Mutex
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	config     config.ServerConfig
}

func (s *SFTPClient) Connect(cfg config.ServerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = cfg
	sshConfig := &ssh.ClientConfig{
		User: cfg.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(cfg.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         12 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	conn, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("SFTP SSH connection failed to %s: %w", addr, err)
	}

	sc, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SFTP client initialization failed: %w", err)
	}

	s.sshClient = conn
	s.sftpClient = sc
	return nil
}

func (s *SFTPClient) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sftpClient != nil {
		_ = s.sftpClient.Close()
		s.sftpClient = nil
	}
	if s.sshClient != nil {
		_ = s.sshClient.Close()
		s.sshClient = nil
	}
	return nil
}

func (s *SFTPClient) ListDirectory(dirPath string) ([]RemoteItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sftpClient == nil {
		return nil, fmt.Errorf("SFTP client is not connected")
	}

	files, err := s.sftpClient.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote directory %s: %w", dirPath, err)
	}

	var items []RemoteItem
	for _, f := range files {
		name := f.Name()
		if name == "." || name == ".." || strings.HasPrefix(name, ".") {
			continue
		}

		fullPath := path.Join(dirPath, name)
		item := ParseFolderName(name, fullPath, f.IsDir())
		if !f.IsDir() {
			item.SizeBytes = f.Size()
		}
		items = append(items, item)
	}

	return items, nil
}

func (s *SFTPClient) ScanRepository(rootPath string) ([]RemoteItem, error) {
	return scanClientRepository(s, rootPath)
}

func (s *SFTPClient) OpenRead(remoteFilePath string, offset int64) (io.ReadCloser, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sftpClient == nil {
		return nil, 0, fmt.Errorf("SFTP client not connected")
	}

	file, err := s.sftpClient.Open(remoteFilePath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open remote file %s: %w", remoteFilePath, err)
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, fmt.Errorf("failed to stat remote file %s: %w", remoteFilePath, err)
	}

	if offset > 0 {
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			file.Close()
			return nil, 0, fmt.Errorf("failed to seek remote file to offset %d: %w", offset, err)
		}
	}

	return file, stat.Size(), nil
}

func (s *SFTPClient) GetFileSize(remoteFilePath string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sftpClient == nil {
		return 0, fmt.Errorf("SFTP client not connected")
	}

	stat, err := s.sftpClient.Stat(remoteFilePath)
	if err != nil {
		return 0, err
	}
	return stat.Size(), nil
}

func (s *SFTPClient) ListFilesRecursive(dirPath string) ([]RemoteFileEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sftpClient == nil {
		return nil, fmt.Errorf("SFTP client not connected")
	}

	var results []RemoteFileEntry
	walker := s.sftpClient.Walk(dirPath)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			continue
		}
		stat := walker.Stat()
		if stat == nil || stat.IsDir() {
			continue
		}

		fullPath := walker.Path()
		relPath := strings.TrimPrefix(fullPath, dirPath)
		relPath = strings.TrimPrefix(relPath, "/")
		relPath = strings.TrimPrefix(relPath, "\\")
		relPath = filepath.FromSlash(relPath)

		results = append(results, RemoteFileEntry{
			RelativePath: relPath,
			FullPath:     fullPath,
			SizeBytes:    stat.Size(),
			ModTime:      stat.ModTime().Unix(),
		})
	}

	return results, nil
}

// ==========================================
// FTP / FTPS Client Implementation
// ==========================================

type FTPClient struct {
	mu        sync.Mutex
	ftpClient *ftp.ServerConn
	config    config.ServerConfig
}

func (f *FTPClient) Connect(cfg config.ServerConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.config = cfg
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	// Try standard FTP dial with TLS fallback
	c, err := ftp.Dial(addr,
		ftp.DialWithTimeout(12*time.Second),
		ftp.DialWithTLS(&tls.Config{InsecureSkipVerify: true}),
	)
	if err != nil {
		// Try unencrypted dial
		c, err = ftp.Dial(addr, ftp.DialWithTimeout(12*time.Second))
		if err != nil {
			return fmt.Errorf("FTP connection failed to %s: %w", addr, err)
		}
	}

	if err := c.Login(cfg.User, cfg.Password); err != nil {
		c.Quit()
		return fmt.Errorf("FTP login failed for user %s: %w", cfg.User, err)
	}

	f.ftpClient = c
	return nil
}

func (f *FTPClient) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.ftpClient != nil {
		_ = f.ftpClient.Quit()
		f.ftpClient = nil
	}
	return nil
}

func (f *FTPClient) ListDirectory(dirPath string) ([]RemoteItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.ftpClient == nil {
		return nil, fmt.Errorf("FTP client not connected")
	}

	entries, err := f.ftpClient.List(dirPath)
	if err != nil {
		return nil, fmt.Errorf("FTP list directory failed for %s: %w", dirPath, err)
	}

	var items []RemoteItem
	for _, entry := range entries {
		name := entry.Name
		if name == "." || name == ".." || strings.HasPrefix(name, ".") {
			continue
		}

		isDir := entry.Type == ftp.EntryTypeFolder
		fullPath := path.Join(dirPath, name)
		item := ParseFolderName(name, fullPath, isDir)
		if !isDir {
			item.SizeBytes = int64(entry.Size)
		}
		items = append(items, item)
	}

	return items, nil
}

func (f *FTPClient) ScanRepository(rootPath string) ([]RemoteItem, error) {
	return scanClientRepository(f, rootPath)
}

func (f *FTPClient) OpenRead(remoteFilePath string, offset int64) (io.ReadCloser, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.ftpClient == nil {
		return nil, 0, fmt.Errorf("FTP client not connected")
	}

	var reader *ftp.Response
	var err error

	if offset > 0 {
		reader, err = f.ftpClient.RetrFrom(remoteFilePath, uint64(offset))
	} else {
		reader, err = f.ftpClient.Retr(remoteFilePath)
	}

	if err != nil {
		return nil, 0, fmt.Errorf("FTP retr failed for %s: %w", remoteFilePath, err)
	}

	size, _ := f.ftpClient.FileSize(remoteFilePath)
	return reader, size, nil
}

func (f *FTPClient) GetFileSize(remoteFilePath string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.ftpClient == nil {
		return 0, fmt.Errorf("FTP client not connected")
	}

	return f.ftpClient.FileSize(remoteFilePath)
}

func (f *FTPClient) ListFilesRecursive(dirPath string) ([]RemoteFileEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.ftpClient == nil {
		return nil, fmt.Errorf("FTP client not connected")
	}

	var results []RemoteFileEntry
	var walk func(currentPath string) error

	walk = func(currentPath string) error {
		entries, err := f.ftpClient.List(currentPath)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if entry.Name == "." || entry.Name == ".." {
				continue
			}
			fullPath := path.Join(currentPath, entry.Name)
			if entry.Type == ftp.EntryTypeFolder {
				_ = walk(fullPath)
			} else {
				relPath := strings.TrimPrefix(fullPath, dirPath)
				relPath = strings.TrimPrefix(relPath, "/")
				relPath = strings.TrimPrefix(relPath, "\\")
				relPath = filepath.FromSlash(relPath)
				results = append(results, RemoteFileEntry{
					RelativePath: relPath,
					FullPath:     fullPath,
					SizeBytes:    int64(entry.Size),
					ModTime:      entry.Time.Unix(),
				})
			}
		}
		return nil
	}

	if err := walk(dirPath); err != nil {
		return nil, err
	}

	return results, nil
}

// TestServerConnection tests whether an SFTP or FTP connection is reachable with auto-fallback
func TestServerConnection(cfg config.ServerConfig) (string, error) {
	// First try configured protocol
	client := NewRemoteClient(cfg.Protocol)
	err := client.Connect(cfg)
	if err == nil {
		_ = client.Close()
		return cfg.Protocol, nil
	}

	// Try fallback protocol
	altProtocol := "sftp"
	altPort := 2022
	if cfg.Protocol == "sftp" {
		altProtocol = "ftp"
		altPort = 21
	}

	altCfg := cfg
	altCfg.Protocol = altProtocol
	altCfg.Port = altPort

	altClient := NewRemoteClient(altProtocol)
	if altErr := altClient.Connect(altCfg); altErr == nil {
		_ = altClient.Close()
		return altProtocol, nil
	}

	// Also check raw TCP ping
	tcpAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	conn, tcpErr := net.DialTimeout("tcp", tcpAddr, 4*time.Second)
	if tcpErr != nil {
		return "", fmt.Errorf("host unreachable on %s: %w", tcpAddr, tcpErr)
	}
	conn.Close()

	return "", fmt.Errorf("authentication or protocol handshake failed: %w", err)
}

// isSectionContainer checks if a folder is a repository section/bucket (e.g. public, private, games, etc.)
func isSectionContainer(name string) bool {
	clean := strings.ToLower(strings.TrimSpace(name))
	switch clean {
	case "public", "private", "games", "game", "releases", "shared", "vip", "torrents",
		"distr", "iso", "content", "repacks", "steam", "pc", "pc games", "pc_games",
		"library", "files", "incoming", "downloads":
		return true
	default:
		return false
	}
}

// scanClientRepository performs multi-section scanning across /public, /private, and root directories
func scanClientRepository(client RemoteClient, rootPath string) ([]RemoteItem, error) {
	rootCandidates := []string{"/", "/public", "/private"}
	cleaned := strings.TrimSpace(rootPath)
	if cleaned != "" && cleaned != "/0" && cleaned != "0" && cleaned != "." {
		rootCandidates = append([]string{cleaned}, rootCandidates...)
	}

	seenPaths := make(map[string]bool)
	var allGames []RemoteItem

	addGame := func(item RemoteItem) {
		if item.RemotePath == "" || seenPaths[item.RemotePath] {
			return
		}
		seenPaths[item.RemotePath] = true
		allGames = append(allGames, item)
	}

	var sectionsToScan []RemoteItem
	scannedDirs := make(map[string]bool)

	for _, cand := range rootCandidates {
		if scannedDirs[cand] {
			continue
		}
		scannedDirs[cand] = true

		items, err := client.ListDirectory(cand)
		if err != nil || len(items) == 0 {
			continue
		}

		for _, item := range items {
			if !item.IsDirectory {
				addGame(item)
				continue
			}

			if isSectionContainer(item.CleanTitle) || isSectionContainer(item.RawName) {
				sectionsToScan = append(sectionsToScan, item)
			} else if item.IsCollection {
				subItems, subErr := client.ListDirectory(item.RemotePath)
				if subErr == nil && len(subItems) > 0 {
					for _, sub := range subItems {
						sub.ParentPath = item.CleanTitle
						addGame(sub)
					}
				} else {
					addGame(item)
				}
			} else {
				addGame(item)
			}
		}
	}

	for _, sec := range sectionsToScan {
		if scannedDirs[sec.RemotePath] {
			continue
		}
		scannedDirs[sec.RemotePath] = true

		secItems, err := client.ListDirectory(sec.RemotePath)
		if err != nil || len(secItems) == 0 {
			continue
		}

		secTag := strings.ToUpper(sec.CleanTitle)

		for _, item := range secItems {
			if item.IsCollection && item.IsDirectory {
				subItems, subErr := client.ListDirectory(item.RemotePath)
				if subErr == nil && len(subItems) > 0 {
					for _, sub := range subItems {
						sub.ParentPath = fmt.Sprintf("[%s] %s", secTag, item.CleanTitle)
						addGame(sub)
					}
					continue
				}
			}

			if item.ParentPath == "" {
				item.ParentPath = secTag
			}
			addGame(item)
		}
	}

	return allGames, nil
}
