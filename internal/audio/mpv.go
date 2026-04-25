package audio

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
)

// defaultIPCBuf is the maximum line size the scanner is willing to grow to.
// 64 KiB (the bufio default) is too small for streams that pack large
// album-art payloads or verbose metadata into a single property change
// (plan §9 mpv IPC pitfall).
const defaultIPCBuf = 1 << 20 // 1 MiB

var errClientClosed = errors.New("mpv ipc: client closed")

// ipcClient owns a single mpv JSON-IPC connection.
//
// All public methods are goroutine-safe: writes are serialized through a
// mutex, and responses are demultiplexed back to the caller waiting on
// their request_id.
type ipcClient struct {
	conn    net.Conn
	writeMu sync.Mutex
	nextID  atomic.Int64
	pending sync.Map // int64 -> chan ipcResponse

	events    chan ipcEvent
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

type ipcRequest struct {
	Command   []any `json:"command"`
	RequestID int64 `json:"request_id"`
}

type ipcResponse struct {
	RequestID int64           `json:"request_id"`
	Data      json.RawMessage `json:"data,omitempty"`
	Error     string          `json:"error"`
}

// ipcEvent is a single event from mpv. Property changes carry Name/Data;
// end-file carries Reason. Unrecognized fields are dropped.
type ipcEvent struct {
	Event  string          `json:"event"`
	ID     int             `json:"id,omitempty"`
	Name   string          `json:"name,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
	Reason string          `json:"reason,omitempty"`
}

// ipcMessage is the union shape we decode into. We branch on which fields
// are populated to decide whether it's an event or a response.
type ipcMessage struct {
	Event     string          `json:"event"`
	ID        int             `json:"id"`
	Name      string          `json:"name"`
	Data      json.RawMessage `json:"data"`
	Reason    string          `json:"reason"`
	RequestID int64           `json:"request_id"`
	Error     string          `json:"error"`
}

// newIPC wraps an established connection with an ipcClient and starts the
// read loop. The connection is closed when (*ipcClient).close is called.
func newIPC(conn net.Conn) *ipcClient {
	return newIPCWithBuf(conn, defaultIPCBuf)
}

func newIPCWithBuf(conn net.Conn, maxBuf int) *ipcClient {
	c := &ipcClient{
		conn:   conn,
		events: make(chan ipcEvent, 64),
		done:   make(chan struct{}),
	}
	go c.readLoop(maxBuf)
	return c
}

// dialIPC connects to an mpv socket already listening at the given path.
// Caller is responsible for spawning mpv and waiting for the socket to
// appear before calling.
func dialIPC(socketPath string) (*ipcClient, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("dial mpv socket: %w", err)
	}
	return newIPC(conn), nil
}

// command sends a JSON-IPC command and waits for the matched response.
// args is the command name followed by its arguments, e.g. command(ctx,
// "set_property", "volume", 50).
func (c *ipcClient) command(ctx context.Context, args ...any) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	id := c.nextID.Add(1)
	ch := make(chan ipcResponse, 1)
	c.pending.Store(id, ch)
	defer c.pending.Delete(id)

	payload, err := json.Marshal(ipcRequest{Command: args, RequestID: id})
	if err != nil {
		return nil, fmt.Errorf("marshal command: %w", err)
	}
	payload = append(payload, '\n')

	c.writeMu.Lock()
	_, err = c.conn.Write(payload)
	c.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("write command: %w", err)
	}

	select {
	case resp := <-ch:
		if resp.Error != "" && resp.Error != "success" {
			return nil, fmt.Errorf("mpv: %s", resp.Error)
		}
		return resp.Data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, errClientClosed
	}
}

// observe subscribes to property change notifications for name. Subsequent
// changes arrive on Events as event="property-change" with id and name set.
func (c *ipcClient) observe(ctx context.Context, propertyID int, name string) error {
	_, err := c.command(ctx, "observe_property", propertyID, name)
	return err
}

// Events returns the receive-only channel of mpv events. The channel is
// closed when the client is closed or the underlying connection ends.
func (c *ipcClient) Events() <-chan ipcEvent {
	return c.events
}

func (c *ipcClient) close() error {
	c.closeOnce.Do(func() {
		close(c.done)
		c.closeErr = c.conn.Close()
	})
	return c.closeErr
}

func (c *ipcClient) readLoop(maxBuf int) {
	defer close(c.events)
	scanner := bufio.NewScanner(c.conn)
	scanner.Buffer(make([]byte, 64*1024), maxBuf)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg ipcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			// Garbage in the stream isn't fatal — mpv occasionally
			// surfaces lines we don't model. Skip and keep reading.
			continue
		}
		switch {
		case msg.Event != "":
			evt := ipcEvent{
				Event:  msg.Event,
				ID:     msg.ID,
				Name:   msg.Name,
				Data:   msg.Data,
				Reason: msg.Reason,
			}
			select {
			case c.events <- evt:
			case <-c.done:
				return
			}
		case msg.RequestID != 0:
			if ch, ok := c.pending.LoadAndDelete(msg.RequestID); ok {
				ch.(chan ipcResponse) <- ipcResponse{
					RequestID: msg.RequestID,
					Data:      msg.Data,
					Error:     msg.Error,
				}
			}
		}
	}
}
