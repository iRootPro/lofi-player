package audio

import (
	"bytes"
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
	exited    *processWaiter
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

	var stderr bytes.Buffer
	cmd := exec.Command("mpv", ambientMPVArgs(socketPath, filePath)...)
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		os.RemoveAll(socketDir)
		return nil, fmt.Errorf("start mpv: %w", err)
	}

	exited := newProcessWaiter(cmd)

	if err := waitForSocketOrExit(ctx, socketPath, 5*time.Second, exited); err != nil {
		terminateMPVProcess(cmd, exited)
		os.RemoveAll(socketDir)
		return nil, formatMPVStartupError(err, stderr.String(), cmd.Args)
	}

	ipc, err := dialIPC(socketPath)
	if err != nil {
		terminateMPVProcess(cmd, exited)
		os.RemoveAll(socketDir)
		return nil, fmt.Errorf("dial mpv: %w", err)
	}
	return &ambientPlayer{cmd: cmd, exited: exited, ipc: ipc, socketDir: socketDir}, nil
}

func (p *ambientPlayer) setVolume(v int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := p.ipc.command(ctx, "set_property", "volume", clampVolume(v))
	return err
}

func (p *ambientPlayer) setPaused(paused bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
		waitForMPVProcessExit(p.cmd, p.exited, 2*time.Second)
		if p.socketDir != "" {
			_ = os.RemoveAll(p.socketDir)
		}
	})
}
