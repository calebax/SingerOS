package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// BridgeProcess manages the Node.js WhatsApp bridge child process.
//
// Implements core.ManagedProcess for lifecycle control and health monitoring.
type BridgeProcess struct {
	scriptPath string
	port       int
	sessionPath string

	cmd *exec.Cmd
	mu  sync.Mutex
}

// NewBridgeProcess creates a bridge process manager.
func NewBridgeProcess(scriptPath string, port int, sessionPath string) *BridgeProcess {
	return &BridgeProcess{
		scriptPath:  scriptPath,
		port:        port,
		sessionPath: sessionPath,
	}
}

// Pid returns the child process ID, or -1 if not running.
func (b *BridgeProcess) Pid() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cmd == nil || b.cmd.Process == nil {
		return -1
	}
	return b.cmd.Process.Pid
}

// Start launches the Node bridge process and waits for health check.
func (b *BridgeProcess) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.cmd != nil {
		return fmt.Errorf("bridge already running (pid=%d)", b.cmd.Process.Pid)
	}

	// Check prerequisites
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("node not found in PATH: %w", err)
	}

	if _, err := os.Stat(b.scriptPath); err != nil {
		return fmt.Errorf("bridge script not found at %s: %w", b.scriptPath, err)
	}

	credsFile := b.credentialPath()
	if _, err := os.Stat(credsFile); err != nil {
		return fmt.Errorf("whatsapp credentials not found at %s (not paired)", credsFile)
	}

	b.cmd = exec.CommandContext(ctx, nodeBin, b.scriptPath,
		"--port", fmt.Sprintf("%d", b.port),
		"--session", b.sessionPath,
	)
	b.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := b.cmd.Start(); err != nil {
		b.cmd = nil
		return fmt.Errorf("start bridge: %w", err)
	}

	// Wait for bridge to become healthy
	bridgectx, cancel := context.WithTimeout(ctx, bridgeStartTimeout)
	defer cancel()

	for {
		select {
		case <-bridgectx.Done():
			b.Stop(ctx)
			return fmt.Errorf("bridge health check timed out")
		case <-time.After(bridgePollInterval):
			if b.isHealthy(ctx) {
				return nil
			}
		}
	}
}

// Stop terminates the bridge process.
func (b *BridgeProcess) Stop(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.cmd == nil || b.cmd.Process == nil {
		return nil
	}

	// Send SIGTERM, wait, then SIGKILL
	if err := b.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		b.cmd.Process.Kill()
	}

	done := make(chan struct{})
	go func() {
		b.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		b.cmd.Process.Kill()
		<-done
	}

	b.cmd = nil
	return nil
}

// Restart stops and starts the bridge.
func (b *BridgeProcess) Restart(ctx context.Context) error {
	if err := b.Stop(ctx); err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	return b.Start(ctx)
}

// isHealthy checks the bridge health endpoint.
func (b *BridgeProcess) isHealthy(ctx context.Context) bool {
	url := fmt.Sprintf("http://localhost:%d%s", b.port, bridgeHealthPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var health BridgeHealth
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return false
	}
	return health.Status == "ok"
}

func (b *BridgeProcess) credentialPath() string {
	return b.sessionPath + "/creds.json"
}
