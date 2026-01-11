package registry

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

var (
	ErrHashMismatch   = errors.New("SHA256 hash mismatch")
	ErrInvalidArchive = errors.New("invalid archive format")
	ErrValidationFailed = errors.New("engine validation failed")
)

// DownloadProgress reports download progress.
type DownloadProgress struct {
	EngineID   string `json:"engineId"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Percent    int    `json:"percent"`
}

// InstallProgress reports overall installation progress.
type InstallProgress struct {
	EngineID string `json:"engineId"`
	Stage    string `json:"stage"` // "downloading", "verifying", "extracting", "validating", "done", "error"
	Message  string `json:"message"`
}

// Installer handles downloading and installing engines.
type Installer struct {
	manager    *Manager
	httpClient *http.Client
	installDir string

	onDownloadProgress func(DownloadProgress)
	onInstallProgress  func(InstallProgress)
}

// NewInstaller creates a new engine installer.
func NewInstaller(manager *Manager) (*Installer, error) {
	installDir, err := InstallDir()
	if err != nil {
		return nil, err
	}

	return &Installer{
		manager:    manager,
		httpClient: &http.Client{Timeout: 30 * time.Minute},
		installDir: installDir,
	}, nil
}

// SetDownloadProgressCallback sets the callback for download progress updates.
func (i *Installer) SetDownloadProgressCallback(cb func(DownloadProgress)) {
	i.onDownloadProgress = cb
}

// SetInstallProgressCallback sets the callback for installation stage updates.
func (i *Installer) SetInstallProgressCallback(cb func(InstallProgress)) {
	i.onInstallProgress = cb
}

// Install downloads, verifies, and installs an engine.
func (i *Installer) Install(ctx context.Context, engineID string) (*InstalledEngine, error) {
	i.emitProgress(engineID, "downloading", "Starting download")

	engine, err := i.manager.GetEngine(engineID)
	if err != nil {
		i.emitProgress(engineID, "error", err.Error())
		return nil, err
	}

	build, buildKey, err := i.manager.SelectBuild(engine)
	if err != nil {
		i.emitProgress(engineID, "error", err.Error())
		return nil, err
	}

	// Create engine directory
	engineDir := filepath.Join(i.installDir, engineID)
	if err := os.MkdirAll(engineDir, 0755); err != nil {
		i.emitProgress(engineID, "error", err.Error())
		return nil, fmt.Errorf("create engine dir: %w", err)
	}

	// Download to temp file
	tempFile := filepath.Join(engineDir, "download.tmp")
	if err := i.download(ctx, engineID, build.URL, tempFile); err != nil {
		os.Remove(tempFile)
		i.emitProgress(engineID, "error", err.Error())
		return nil, err
	}

	// Verify hash
	i.emitProgress(engineID, "verifying", "Verifying SHA256 hash")
	if err := i.verifyHash(tempFile, build.SHA256); err != nil {
		os.Remove(tempFile)
		i.emitProgress(engineID, "error", err.Error())
		return nil, err
	}

	// Extract
	i.emitProgress(engineID, "extracting", "Extracting archive")
	binaryPath, err := i.extract(tempFile, engineDir, build.Binary, build.Extract)
	os.Remove(tempFile)
	if err != nil {
		i.emitProgress(engineID, "error", err.Error())
		return nil, err
	}

	// Set executable permissions on Unix
	if err := os.Chmod(binaryPath, 0755); err != nil {
		i.emitProgress(engineID, "error", err.Error())
		return nil, fmt.Errorf("chmod: %w", err)
	}

	// Validate engine
	i.emitProgress(engineID, "validating", "Validating engine")
	if err := i.validate(ctx, binaryPath); err != nil {
		i.emitProgress(engineID, "error", err.Error())
		return nil, err
	}

	// Save config
	installed := &InstalledEngine{
		ID:          engineID,
		RegistryID:  engineID,
		Name:        engine.Name,
		Version:     engine.Version,
		BinaryPath:  binaryPath,
		InstalledAt: time.Now().Format(time.RFC3339),
		BuildKey:    buildKey,
	}

	configPath := filepath.Join(engineDir, "config.toml")
	if err := i.saveConfig(configPath, installed); err != nil {
		i.emitProgress(engineID, "error", err.Error())
		return nil, err
	}

	i.emitProgress(engineID, "done", "Installation complete")
	return installed, nil
}

// download fetches a URL to a local file with progress reporting.
func (i *Installer) download(ctx context.Context, engineID, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	total := resp.ContentLength
	var downloaded int64

	buf := make([]byte, 32*1024)
	lastReport := time.Now()

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write file: %w", writeErr)
			}
			downloaded += int64(n)

			// Report progress at most every 100ms
			if time.Since(lastReport) > 100*time.Millisecond {
				i.emitDownloadProgress(engineID, downloaded, total)
				lastReport = time.Now()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("download read: %w", err)
		}
	}

	// Final progress report
	i.emitDownloadProgress(engineID, downloaded, total)
	return nil
}

// verifyHash checks the SHA256 hash of a file.
func (i *Installer) verifyHash(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("%w: expected %s, got %s", ErrHashMismatch, expected, actual)
	}

	return nil
}

// extract extracts an archive and returns the path to the binary.
func (i *Installer) extract(archivePath, destDir, binaryName, format string) (string, error) {
	switch format {
	case "zip":
		return i.extractZip(archivePath, destDir, binaryName)
	case "tar":
		return i.extractTar(archivePath, destDir, binaryName, false)
	case "tar.gz", "tgz":
		return i.extractTar(archivePath, destDir, binaryName, true)
	case "":
		// Raw binary, just move it
		binaryPath := filepath.Join(destDir, filepath.Base(binaryName))
		if err := os.Rename(archivePath, binaryPath); err != nil {
			return "", err
		}
		return binaryPath, nil
	default:
		return "", fmt.Errorf("%w: unknown format %s", ErrInvalidArchive, format)
	}
}

func (i *Installer) extractZip(archivePath, destDir, binaryName string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	var binaryPath string
	for _, f := range r.File {
		destPath := filepath.Join(destDir, f.Name)

		// Security check: prevent path traversal
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(destPath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return "", err
		}

		outFile, err := os.Create(destPath)
		if err != nil {
			return "", err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return "", err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return "", err
		}

		// Check if this is the binary we're looking for
		if strings.HasSuffix(f.Name, binaryName) || f.Name == binaryName {
			binaryPath = destPath
		}
	}

	if binaryPath == "" {
		return "", fmt.Errorf("binary %s not found in archive", binaryName)
	}
	return binaryPath, nil
}

func (i *Installer) extractTar(archivePath, destDir, binaryName string, gzipped bool) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var reader io.Reader = f
	if gzipped {
		gr, err := gzip.NewReader(f)
		if err != nil {
			return "", fmt.Errorf("gzip reader: %w", err)
		}
		defer gr.Close()
		reader = gr
	}

	tr := tar.NewReader(reader)
	var binaryPath string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar read: %w", err)
		}

		destPath := filepath.Join(destDir, header.Name)

		// Security check: prevent path traversal
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(destPath, 0755)
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return "", err
			}

			outFile, err := os.Create(destPath)
			if err != nil {
				return "", err
			}

			_, err = io.Copy(outFile, tr)
			outFile.Close()
			if err != nil {
				return "", err
			}

			// Check if this is the binary we're looking for
			if strings.HasSuffix(header.Name, binaryName) || header.Name == binaryName {
				binaryPath = destPath
			}
		}
	}

	if binaryPath == "" {
		return "", fmt.Errorf("binary %s not found in archive", binaryName)
	}
	return binaryPath, nil
}

// validate runs the engine and checks for uciok response.
func (i *Installer) validate(ctx context.Context, binaryPath string) error {
	// Use the existing UCI engine code to validate
	// Import would create a cycle, so we do basic validation here
	cmd := execCommandContext(ctx, binaryPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: failed to start engine: %v", ErrValidationFailed, err)
	}

	// Send uci command
	fmt.Fprintln(stdin, "uci")

	// Wait for uciok with timeout
	done := make(chan bool)
	var uciok bool

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				break
			}
			if strings.Contains(string(buf[:n]), "uciok") {
				uciok = true
				break
			}
		}
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}

	// Cleanup
	fmt.Fprintln(stdin, "quit")
	stdin.Close()
	cmd.Wait()

	if !uciok {
		return fmt.Errorf("%w: engine did not respond with uciok", ErrValidationFailed)
	}

	return nil
}

// saveConfig writes the installed engine configuration.
func (i *Installer) saveConfig(path string, installed *InstalledEngine) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(installed)
}

// emitProgress sends an installation progress update.
func (i *Installer) emitProgress(engineID, stage, message string) {
	if i.onInstallProgress != nil {
		i.onInstallProgress(InstallProgress{
			EngineID: engineID,
			Stage:    stage,
			Message:  message,
		})
	}
}

// emitDownloadProgress sends a download progress update.
func (i *Installer) emitDownloadProgress(engineID string, downloaded, total int64) {
	if i.onDownloadProgress != nil {
		percent := 0
		if total > 0 {
			percent = int(downloaded * 100 / total)
		}
		i.onDownloadProgress(DownloadProgress{
			EngineID:   engineID,
			Downloaded: downloaded,
			Total:      total,
			Percent:    percent,
		})
	}
}

// ListInstalled returns all installed engines.
func (i *Installer) ListInstalled() ([]InstalledEngine, error) {
	entries, err := os.ReadDir(i.installDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var installed []InstalledEngine
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		configPath := filepath.Join(i.installDir, entry.Name(), "config.toml")
		data, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}

		var eng InstalledEngine
		if err := toml.Unmarshal(data, &eng); err != nil {
			continue
		}

		installed = append(installed, eng)
	}

	return installed, nil
}

// GetInstalled returns an installed engine by ID.
func (i *Installer) GetInstalled(engineID string) (*InstalledEngine, error) {
	configPath := filepath.Join(i.installDir, engineID, "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrEngineNotFound
		}
		return nil, err
	}

	var eng InstalledEngine
	if err := toml.Unmarshal(data, &eng); err != nil {
		return nil, err
	}

	return &eng, nil
}

// Uninstall removes an installed engine.
func (i *Installer) Uninstall(engineID string) error {
	engineDir := filepath.Join(i.installDir, engineID)
	return os.RemoveAll(engineDir)
}

// execCommandContext creates an exec.Cmd for engine validation.
var execCommandContext = func(ctx context.Context, name string) *execCmd {
	return &execCmd{ctx: ctx, name: name}
}

// execCmd wraps os/exec.Cmd for engine validation.
type execCmd struct {
	ctx    context.Context
	name   string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (c *execCmd) StdinPipe() (io.WriteCloser, error) {
	c.cmd = exec.CommandContext(c.ctx, c.name)
	var err error
	c.stdin, err = c.cmd.StdinPipe()
	return c.stdin, err
}

func (c *execCmd) StdoutPipe() (io.ReadCloser, error) {
	if c.cmd == nil {
		c.cmd = exec.CommandContext(c.ctx, c.name)
	}
	var err error
	c.stdout, err = c.cmd.StdoutPipe()
	return c.stdout, err
}

func (c *execCmd) Start() error {
	return c.cmd.Start()
}

func (c *execCmd) Wait() error {
	return c.cmd.Wait()
}
