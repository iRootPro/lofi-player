package audio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type ambientPlayer struct {
	cmd       *exec.Cmd
	ipc       *ipcClient
	socketDir string
	closeOnce sync.Once
}

func newAmbientPlayer(ctx context.Context, filePath string) (*ambientPlayer, error) {
	socketDir, err := os.MkdirTemp("/tmp", "lofi-ambient-*")
	if err != nil {
		socketDir, err = os.MkdirTemp("", "lofi-ambient-*")
		if err != nil {
			return nil, fmt.Errorf("create socket dir: %w", err)
		}
	}
	socketPath := filepath.Join(socketDir, "mpv.sock")

	cmd := exec.Command("mpv",
		"--idle=no",
		"--no-video",
		"--no-terminal",
		"--really-quiet",
		"--loop-file=inf",
		"--volume=0",
		"--pause=yes",
		"--input-ipc-server="+socketPath,
		filePath,
	)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(socketDir)
		return nil, fmt.Errorf("start mpv: %w", err)
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	if err := waitForSocketOrExit(ctx, socketPath, 5*time.Second, exited); err != nil {
		// waitForSocketOrExit may have consumed the exit value from `exited`
		// when mpv died early, so a follow-up <-exited would block forever.
		// Best-effort kill is enough; the goroutine sends to a buffered
		// channel (size 1) and exits cleanly once cmd.Wait returns.
		_ = cmd.Process.Kill()
		os.RemoveAll(socketDir)
		return nil, fmt.Errorf("mpv socket did not appear: %w", err)
	}

	ipc, err := dialIPC(socketPath)
	if err != nil {
		_ = cmd.Process.Kill()
		os.RemoveAll(socketDir)
		return nil, fmt.Errorf("dial mpv: %w", err)
	}
	return &ambientPlayer{cmd: cmd, ipc: ipc, socketDir: socketDir}, nil
}

func (p *ambientPlayer) setVolume(v int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := p.ipc.command(ctx, "set_property", "volume", clampVolume(v))
	return err
}

func (p *ambientPlayer) setPaused(paused bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := p.ipc.command(ctx, "set_property", "pause", paused)
	return err
}

func (p *ambientPlayer) close() {
	p.closeOnce.Do(func() {
		if p.ipc != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			_, _ = p.ipc.command(ctx, "quit")
			cancel()
			_ = p.ipc.close()
		}
		if p.cmd != nil && p.cmd.Process != nil {
			done := make(chan struct{})
			go func() { _ = p.cmd.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				_ = p.cmd.Process.Kill()
				<-done
			}
		}
		if p.socketDir != "" {
			_ = os.RemoveAll(p.socketDir)
		}
	})
}
