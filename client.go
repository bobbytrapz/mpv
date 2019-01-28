package mpv

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:generate go run gen/gen.go

// Client controls mpv over unix socket
type Client struct {
	cmd  *exec.Cmd
	conn net.Conn
	ipc  *bufio.ReadWriter
}

// Reply to a mpv request
// { "error": "success", "data": null }
// { "event": "event_name" }
type Reply struct {
	Event     string      `json:"event"`
	Error     string      `json:"error"`
	Data      interface{} `json:"data"`
	RequestID int         `json:"request_id"`
}

var requests = make(map[int]chan Reply)
var rw sync.RWMutex
var wg sync.WaitGroup
var nextReqID int

// Log function
var Log = func(string, ...interface{}) {}

// Closed recv when mpv after closed
var Closed = make(chan struct{}, 1)

func getReq(id int) (ch chan Reply, ok bool) {
	rw.RLock()
	ch, ok = requests[id]
	rw.RUnlock()

	return
}

func addReq(ch chan Reply) {
	rw.Lock()
	requests[nextReqID] = ch
	nextReqID++
	rw.Unlock()
}

func delReq(id int) {
	rw.Lock()
	delete(requests, id)
	rw.Unlock()
}

// WaitForOpen is how we should wait for mpv to open
const WaitForOpen = 5 * time.Second

// NewClient starts an mpv instance and establishes a connection to it over a unix socket
// the socket is removed if it already exists
func NewClient(ctx context.Context, sockfn string) (client *Client, err error) {
	return NewClientWithFlags(ctx, sockfn, true, "--idle=yes", fmt.Sprintf("--input-unix-socket=%s", sockfn))
}

// NewClientWithFlags starts and mpv with custom flags
// the socket is removed if it already exists
func NewClientWithFlags(ctx context.Context, sockfn string, newProcess bool, flags ...string) (client *Client, err error) {
	return newClient(ctx, sockfn, true, flags...)
}

// NewClientConnect to an existing mpv player socket or wait until it is avaliable
func NewClientConnect(ctx context.Context, sockfn string) (client *Client, err error) {
	return newClient(ctx, sockfn, false)
}

func newClient(ctx context.Context, sockfn string, newProcess bool, flags ...string) (client *Client, err error) {
	var cmd *exec.Cmd
	if newProcess {
		// run mpv
		cmd = exec.Command("mpv", flags...)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
		err = cmd.Start()
		if err != nil {
			err = fmt.Errorf("mpv.newClient: could not start mpv: %s", err)
			return
		}

		// remove socket if it exists
		if _, err := os.Stat(sockfn); !os.IsNotExist(err) {
			os.Remove(sockfn)
			Log("mpv.newClient: removed: %s\n", sockfn)
		}
	}

	// wait for connection
	Log("mpv.newClient: wait for connection...")
	timeout := time.After(WaitForOpen)
	for {
		select {
		case err := <-ctx.Done():
			return nil, fmt.Errorf("mpv.newClient: did not make client: %s", err)
		case <-timeout:
			return nil, errors.New("mpv.newClient: timeout")
		default:
			if _, err := os.Stat(sockfn); !os.IsNotExist(err) {
				Log("mpv.newClient: running mpv (ipc=%s)\n", sockfn)
				goto found
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
found:

	// make client
	conn, err := net.Dial("unix", sockfn)
	if err != nil {
		err = fmt.Errorf("mpv.newClient: could not make client: %s", err)
		return
	}
	client = &Client{
		cmd:  cmd,
		conn: conn,
		ipc:  bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
	}

	// kill when canceled
	go func() {
		<-ctx.Done()
		Log("mpv.Client: %s", ctx.Err())
		client.Kill()
	}()

	// read replies and dispatch through channels waiting for replies
	wg.Add(1)
	go func() {
		// clean up
		defer func() {
			client.conn.Close()
			if newProcess {
				// we made this connection so we should clean it up
				os.Remove(sockfn)
			}
			Log("mpv.Client: closed")
			Closed <- struct{}{}
			wg.Done()
		}()

		for {
			// wait for reply
			data, err := client.ipc.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					return
				}
				Log("mpv.Client: recv: %s", err)
				continue
			}

			var reply Reply
			err = json.Unmarshal(data, &reply)
			if err != nil {
				Log("mpv.Client: %s", err)
				continue
			}

			// handle event
			if reply.Event != "" {
				Log("mpv: event: %s", string(data))
				handleEvent(reply.Event)
			} else {
				// dispatch reply
				ch, ok := getReq(reply.RequestID)
				if ok {
					ch <- reply
					close(ch)
					delReq(reply.RequestID)
				}
			}
		}
	}()

	return
}

// WaitForExit is how long we should wait for mpv to close
var WaitForExit = 2 * time.Second

// Wait for mpv to close
func (c *Client) Wait() error {
	done := make(chan error, 1)
	go func() {
		wg.Wait()
		if c.cmd == nil {
			done <- nil
			return
		}
		done <- c.cmd.Wait()
	}()
	select {
	case <-time.After(WaitForExit):
		Log("mpv.Wait: timeout\n")
		return errors.New("mpv.Wait: timeout")
	case err := <-done:
		return err
	}
}

// Kill running mpv process
func (c *Client) Kill() error {
	if c.cmd == nil {
		// we did not make the mpv process so we will not kill it
		return nil
	}
	Log("mpv.Kill: %d", c.cmd.Process.Pid)
	return c.cmd.Process.Kill()
}

// Execute sends a command over ipc.
// { "command": ["command_name", "param1", "param2", ...] }
func (c *Client) Execute(name string, params ...interface{}) (chan Reply, error) {
	// build command
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("{\"command\":[\"%s\"", name))
	for _, param := range params {
		fmt.Fprintf(&sb, ",\"%s\"", param)
	}
	fmt.Fprintf(&sb, "],\"request_id\":%d}", nextReqID)

	// add reply channel
	ch := make(chan Reply, 1)
	addReq(ch)

	// send command over ipc
	_, err := c.ipc.WriteString(sb.String() + "\n")
	if err != nil {
		return nil, fmt.Errorf("mpv.Execute: %s", err)
	}
	err = c.ipc.Flush()
	if err != nil {
		return nil, fmt.Errorf("mpv.Execute: %s", err)
	}

	return ch, nil
}

// WaitForReply is how long we should wait for mpv to reply
const WaitForReply = 5 * time.Second

// ReplyOrTimeout waits for a reply from mpv or gives an error
func ReplyOrTimeout(ch chan Reply, wait time.Duration) (Reply, error) {
	select {
	case <-time.After(wait):
		return Reply{}, fmt.Errorf("mpv.ReplyOrTimeout: timeout")
	case re := <-ch:
		return re, nil
	}
}

// a few methods for convenience

// LoadFile runs 'loadfile' command
func (c *Client) LoadFile(params ...interface{}) error {
	return c.SendCommand("loadfile", params...)
}

// LoadList runs 'loadlist' command
func (c *Client) LoadList(params ...interface{}) error {
	return c.SendCommand("loadlist", params...)
}

// Version gets 'mpv-version' property
func (c *Client) Version() (version string, err error) {
	p, err := c.GetProperty("mpv-version")
	if err == nil {
		version = p.(string)
	}
	return
}

// AddFile appends to playlist
func (c *Client) AddFile(fn string) error {
	return c.SendCommand("loadfile", fn, "append-play")
}

// PlayFile overwrites playlist with one file
func (c *Client) PlayFile(fn string) error {
	return c.SendCommand("loadfile", fn)
}
