package communication

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHClient manages SSH connections to the helper node
type SSHClient struct {
	host    string
	user    string
	keyFile string
	client  *ssh.Client
	verbose bool
}

// NewSSHClient creates a new SSH client
func NewSSHClient(host, user, keyFile string, verbose bool) *SSHClient {
	return &SSHClient{
		host:    host,
		user:    user,
		keyFile: keyFile,
		verbose: verbose,
	}
}

// Connect establishes SSH connection
func (s *SSHClient) Connect() error {
	if s.verbose {
		fmt.Printf("[SSH] Connecting to %s@%s...\n", s.user, s.host)
	}

	expandedPath := os.ExpandEnv(strings.ReplaceAll(s.keyFile, "~", "$HOME"))
	key, err := os.ReadFile(expandedPath)
	if err != nil {
		return fmt.Errorf("failed to read private key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %v", err)
	}

	config := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", s.host), config)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}

	s.client = client

	if s.verbose {
		fmt.Println("[SSH] ✅ Connected successfully")
	}

	return nil
}

// Close closes the SSH connection
func (s *SSHClient) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// ExecuteCommand executes a command on the remote host
func (s *SSHClient) ExecuteCommand(command string) (string, error) {
	return s.ExecuteCommandCtx(context.Background(), command)
}

// ExecuteCommandCtx executes a command on the remote host with a timeout context
func (s *SSHClient) ExecuteCommandCtx(ctx context.Context, command string) (string, error) {
	if s.client == nil {
		return "", fmt.Errorf("not connected")
	}

	session, err := s.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	if s.verbose {
		fmt.Printf("[SSH] Executing: %s\n", command)
	}

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Listen for context cancellation to kill the session early
	go func() {
		<-ctx.Done()
		if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
			session.Close()
		}
	}()

	err = session.Run(command)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out: %s", command)
		}
		return "", fmt.Errorf("command failed: %v\nStderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// ExecuteCommandWithOutput executes a command and streams output
func (s *SSHClient) ExecuteCommandWithOutput(command string) error {
	if s.client == nil {
		return fmt.Errorf("not connected")
	}

	session, err := s.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	if s.verbose {
		fmt.Printf("[SSH] Executing: %s\n", command)
	}

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	return session.Run(command)
}

// UploadFile uploads a file to the remote host
func (s *SSHClient) UploadFile(localPath, remotePath string) error {
	if s.client == nil {
		return fmt.Errorf("not connected")
	}

	if s.verbose {
		fmt.Printf("[SSH] Uploading %s -> %s\n", localPath, remotePath)
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %v", err)
	}

	remoteDir := filepath.Dir(remotePath)
	if _, err := s.ExecuteCommand(fmt.Sprintf("mkdir -p %s", remoteDir)); err != nil {
		return fmt.Errorf("failed to create remote directory: %v", err)
	}

	session, err := s.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %v", err)
	}

	if err := session.Start(fmt.Sprintf("cat > %s", remotePath)); err != nil {
		return fmt.Errorf("failed to start cat command: %v", err)
	}

	if _, err := stdin.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %v", err)
	}

	stdin.Close()

	if err := session.Wait(); err != nil {
		return fmt.Errorf("failed to wait for command: %v", err)
	}

	if s.verbose {
		fmt.Printf("[SSH] ✅ File uploaded successfully\n")
	}

	return nil
}

// UploadContent uploads content to a remote file
func (s *SSHClient) UploadContent(content, remotePath string) error {
	if s.client == nil {
		return fmt.Errorf("not connected")
	}

	if s.verbose {
		fmt.Printf("[SSH] Uploading content to %s\n", remotePath)
	}

	remoteDir := filepath.Dir(remotePath)
	if _, err := s.ExecuteCommand(fmt.Sprintf("mkdir -p %s", remoteDir)); err != nil {
		return fmt.Errorf("failed to create remote directory: %v", err)
	}

	session, err := s.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %v", err)
	}

	if err := session.Start(fmt.Sprintf("cat > %s", remotePath)); err != nil {
		return fmt.Errorf("failed to start cat command: %v", err)
	}

	if _, err := io.WriteString(stdin, content); err != nil {
		return fmt.Errorf("failed to write content: %v", err)
	}

	stdin.Close()

	if err := session.Wait(); err != nil {
		return fmt.Errorf("failed to wait for command: %v", err)
	}

	if s.verbose {
		fmt.Printf("[SSH] ✅ Content uploaded successfully\n")
	}

	return nil
}

// DownloadFile downloads a file from the remote host
func (s *SSHClient) DownloadFile(remotePath, localPath string) error {
	if s.client == nil {
		return fmt.Errorf("not connected")
	}

	if s.verbose {
		fmt.Printf("[SSH] Downloading %s -> %s\n", remotePath, localPath)
	}

	content, err := s.ExecuteCommand(fmt.Sprintf("cat %s", remotePath))
	if err != nil {
		return fmt.Errorf("failed to read remote file: %v", err)
	}

	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %v", err)
	}

	if err := os.WriteFile(localPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write local file: %v", err)
	}

	if s.verbose {
		fmt.Printf("[SSH] ✅ File downloaded successfully\n")
	}

	return nil
}

// FileExists checks if a file exists on the remote host
func (s *SSHClient) FileExists(remotePath string) (bool, error) {
	if s.client == nil {
		return false, fmt.Errorf("not connected")
	}

	_, err := s.ExecuteCommand(fmt.Sprintf("test -f %s", remotePath))
	if err != nil {
		if strings.Contains(err.Error(), "exit status 1") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// DirectoryExists checks if a directory exists on the remote host
func (s *SSHClient) DirectoryExists(remotePath string) (bool, error) {
	if s.client == nil {
		return false, fmt.Errorf("not connected")
	}

	_, err := s.ExecuteCommand(fmt.Sprintf("test -d %s", remotePath))
	if err != nil {
		if strings.Contains(err.Error(), "exit status 1") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// CreateDirectory creates a directory on the remote host
func (s *SSHClient) CreateDirectory(remotePath string) error {
	if s.verbose {
		fmt.Printf("[SSH] Creating directory: %s\n", remotePath)
	}

	_, err := s.ExecuteCommand(fmt.Sprintf("mkdir -p %s", remotePath))
	return err
}

// RemoveFile removes a file on the remote host
func (s *SSHClient) RemoveFile(remotePath string) error {
	if s.verbose {
		fmt.Printf("[SSH] Removing file: %s\n", remotePath)
	}

	_, err := s.ExecuteCommand(fmt.Sprintf("rm -f %s", remotePath))
	return err
}

// RemoveDirectory removes a directory on the remote host
func (s *SSHClient) RemoveDirectory(remotePath string) error {
	if s.verbose {
		fmt.Printf("[SSH] Removing directory: %s\n", remotePath)
	}

	_, err := s.ExecuteCommand(fmt.Sprintf("rm -rf %s", remotePath))
	return err
}

// SystemctlReload reloads a systemd service
func (s *SSHClient) SystemctlReload(service string) error {
	if s.verbose {
		fmt.Printf("[SSH] Reloading service: %s\n", service)
	}

	_, err := s.ExecuteCommand(fmt.Sprintf("systemctl reload %s", service))
	return err
}

// SystemctlRestart restarts a systemd service
func (s *SSHClient) SystemctlRestart(service string) error {
	if s.verbose {
		fmt.Printf("[SSH] Restarting service: %s\n", service)
	}

	_, err := s.ExecuteCommand(fmt.Sprintf("sudo systemctl restart %s", service))
	return err
}

// SystemctlEnable enables and starts a systemd service
func (s *SSHClient) SystemctlEnable(service string) error {
	if s.verbose {
		fmt.Printf("[SSH] Enabling and starting service: %s\n", service)
	}

	_, err := s.ExecuteCommand(fmt.Sprintf("systemctl enable --now %s", service))
	return err
}

// SystemctlStatus checks the status of a systemd service
func (s *SSHClient) SystemctlStatus(service string) (string, error) {
	return s.ExecuteCommand(fmt.Sprintf("systemctl status %s", service))
}

// RestoreconRecursive runs restorecon -Rv on a path (SELinux)
func (s *SSHClient) RestoreconRecursive(path string) error {
	if s.verbose {
		fmt.Printf("[SSH] Running restorecon -Rv on %s\n", path)
	}

	_, err := s.ExecuteCommand(fmt.Sprintf("restorecon -Rv %s", path))
	return err
}
