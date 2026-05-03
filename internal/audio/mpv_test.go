package audio

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// pipePair returns an ipcClient backed by net.Pipe and the server-side
// connection the test can use to play the role of mpv. Both ends are
// closed via t.Cleanup.
func pipePair(t *testing.T, maxBuf int) (*ipcClient, net.Conn) {
	t.Helper()
	server, client := net.Pipe()
	c := newIPCWithBuf(client, maxBuf)
	t.Cleanup(func() {
		c.close()
		server.Close()
	})
	return c, server
}

func TestCommand_RequestIDCorrelation(t *testing.T) {
	c, server := pipePair(t, defaultIPCBuf)

	type result struct {
		i    int
		data string
		err  error
	}
	results := make(chan result, 2)
	for i := range 2 {
		go func(i int) {
			data, err := c.command(context.Background(), "ping", i)
			results <- result{i, string(data), err}
		}(i)
	}

	// Server side: drain both requests, remembering the i → request_id
	// mapping (i is the second arg in the command array).
	scanner := bufio.NewScanner(server)
	idForI := map[int]int64{}
	for range 2 {
		if !scanner.Scan() {
			t.Fatal("scanner ended before reading both requests")
		}
		var req struct {
			Command   []any `json:"command"`
			RequestID int64 `json:"request_id"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		i := int(req.Command[1].(float64))
		idForI[i] = req.RequestID
	}

	// Respond in reverse order to prove the demuxer routes by request_id,
	// not by fifo.
	for _, i := range []int{1, 0} {
		resp, _ := json.Marshal(ipcResponse{
			RequestID: idForI[i],
			Data:      json.RawMessage(fmt.Sprintf(`"resp-for-i-%d"`, i)),
			Error:     "success",
		})
		server.Write(append(resp, '\n'))
	}

	for range 2 {
		select {
		case r := <-results:
			if r.err != nil {
				t.Errorf("i=%d returned error: %v", r.i, r.err)
				continue
			}
			want := fmt.Sprintf(`"resp-for-i-%d"`, r.i)
			if r.data != want {
				t.Errorf("i=%d got %s, want %s", r.i, r.data, want)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for command result")
		}
	}
}

func TestCommand_ErrorResponseSurfaced(t *testing.T) {
	c, server := pipePair(t, defaultIPCBuf)

	go func() {
		scanner := bufio.NewScanner(server)
		if scanner.Scan() {
			var req ipcRequest
			json.Unmarshal(scanner.Bytes(), &req)
			resp, _ := json.Marshal(ipcResponse{RequestID: req.RequestID, Error: "property unavailable"})
			server.Write(append(resp, '\n'))
		}
	}()

	_, err := c.command(context.Background(), "get_property", "nonexistent")
	if err == nil || !strings.Contains(err.Error(), "property unavailable") {
		t.Errorf("expected error mentioning 'property unavailable', got %v", err)
	}
}

func TestEvents_PropertyChangeForwarded(t *testing.T) {
	c, server := pipePair(t, defaultIPCBuf)

	go func() {
		evt := `{"event":"property-change","id":1,"name":"pause","data":true}` + "\n"
		server.Write([]byte(evt))
	}()

	select {
	case e := <-c.Events():
		if e.Event != "property-change" || e.Name != "pause" {
			t.Errorf("got %+v, want event=property-change name=pause", e)
		}
		if string(e.Data) != "true" {
			t.Errorf("data: got %s, want true", e.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestEvents_EndFileWithReason(t *testing.T) {
	c, server := pipePair(t, defaultIPCBuf)

	go func() {
		server.Write([]byte(`{"event":"end-file","reason":"error"}` + "\n"))
	}()

	select {
	case e := <-c.Events():
		if e.Event != "end-file" || e.Reason != "error" {
			t.Errorf("got %+v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestReadLoop_LargeLineWithinBudget(t *testing.T) {
	c, server := pipePair(t, defaultIPCBuf)

	// 200 KiB title — well past the 64 KiB bufio default but inside our 1 MiB cap.
	huge := strings.Repeat("a", 200*1024)
	line := fmt.Sprintf(`{"event":"property-change","name":"metadata","data":{"icy-title":%q}}`+"\n", huge)
	go func() { server.Write([]byte(line)) }()

	select {
	case e := <-c.Events():
		if e.Name != "metadata" {
			t.Errorf("event name: %q, want metadata", e.Name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout — large line not delivered")
	}
}

func TestReadLoop_MalformedLineSkipped(t *testing.T) {
	c, server := pipePair(t, defaultIPCBuf)

	go func() {
		// Garbage line, then a valid event.
		server.Write([]byte("not even json\n"))
		server.Write([]byte(`{"event":"valid"}` + "\n"))
	}()

	select {
	case e := <-c.Events():
		if e.Event != "valid" {
			t.Errorf("got %+v, want event=valid", e)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout — malformed line should not stop the read loop")
	}
}

func TestCommand_ResponseNotStarvedByFullEventQueue(t *testing.T) {
	c, server := pipePair(t, defaultIPCBuf)

	result := make(chan error, 1)
	go func() {
		_, err := c.command(context.Background(), "set_property", "volume", 55)
		result <- err
	}()

	scanner := bufio.NewScanner(server)
	if !scanner.Scan() {
		t.Fatal("scanner ended before reading request")
	}
	var req ipcRequest
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
		t.Fatalf("decode request: %v", err)
	}

	// Simulate a noisy stream that fills the event queue before mpv's
	// command response arrives. The IPC reader must keep reading and route
	// the response instead of blocking forever on an undrained Events chan.
	writeErr := make(chan error, 1)
	go func() {
		for i := 0; i < cap(c.events)+10; i++ {
			line := fmt.Sprintf(`{"event":"property-change","id":9,"name":"demuxer-cache-state","data":{"fw-bytes":%d}}`+"\n", i)
			if _, err := server.Write([]byte(line)); err != nil {
				writeErr <- fmt.Errorf("write event %d: %w", i, err)
				return
			}
		}
		resp, _ := json.Marshal(ipcResponse{RequestID: req.RequestID, Error: "success"})
		if _, err := server.Write(append(resp, '\n')); err != nil {
			writeErr <- fmt.Errorf("write response: %w", err)
			return
		}
		writeErr <- nil
	}()

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("command returned error: %v", err)
		}
		if err := <-writeErr; err != nil {
			t.Fatal(err)
		}
	case err := <-writeErr:
		if err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-result:
			if err != nil {
				t.Fatalf("command returned error: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for command response after writes completed")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for command response; event backpressure likely starved responses")
	}
}

func TestCommand_ContextCancellation(t *testing.T) {
	c, _ := pipePair(t, defaultIPCBuf)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.command(ctx, "wait_forever")
	if err == nil {
		t.Error("expected context error, got nil")
	}
}

func TestCommand_AfterCloseFails(t *testing.T) {
	c, _ := pipePair(t, defaultIPCBuf)
	c.close()

	_, err := c.command(context.Background(), "anything")
	if err == nil {
		t.Error("expected error from command after close")
	}
}

func TestEvents_ChannelClosesWhenConnEnds(t *testing.T) {
	c, server := pipePair(t, defaultIPCBuf)

	server.Close()

	select {
	case _, ok := <-c.Events():
		if ok {
			t.Error("expected events channel to be closed once conn ends")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout — events channel did not close after server hangup")
	}
}
