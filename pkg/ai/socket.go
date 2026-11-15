// Package ai provides the Unix-socket transports exposed by the AI device.
package ai

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"unicode/utf8"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/ghofuobudgeghikhdrhiehdihebd/airlock/pkg/schemas"
)

const (
	IOSocketPath   = "/tmp/ai-io.socket"
	ToolSocketPath = "/tmp/ai-tool.socket"
)

// IOInterface sends MessagePack requests to the AI device and receives its
// raw, UTF-8 output.
type IOInterface interface {
	Stream(context.Context, schemas.Message, func(string) error) error
}

// IOClient connects to the AI device's I/O socket.
type IOClient struct {
	Path string
}

func NewIOClient() *IOClient { return &IOClient{Path: IOSocketPath} }

// Stream sends a request, then passes each valid UTF-8 output chunk to
// receive. A final nil result means the AI device closed its response stream.
func (c *IOClient) Stream(ctx context.Context, request schemas.Message, receive func(string) error) error {
	payload, err := msgpack.Marshal(request)
	if err != nil {
		return err
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", c.Path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.Write(payload); err != nil {
		return err
	}
	if unixConn, ok := conn.(*net.UnixConn); ok {
		if err := unixConn.CloseWrite(); err != nil {
			return err
		}
	}
	return readUTF8(bufio.NewReader(conn), receive)
}

func readUTF8(reader *bufio.Reader, receive func(string) error) error {
	buffer := make([]byte, 4096)
	pending := []byte(nil)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			data := append(pending, buffer[:n]...)
			complete := 0
			for complete < len(data) {
				if !utf8.FullRune(data[complete:]) {
					break
				}
				runeValue, size := utf8.DecodeRune(data[complete:])
				if runeValue == utf8.RuneError && size == 1 {
					return errors.New("ai: I/O output must be valid UTF-8")
				}
				complete += size
			}
			if complete > 0 && receive != nil {
				if err := receive(string(data[:complete])); err != nil {
					return err
				}
			}
			pending = append(pending[:0], data[complete:]...)
		}
		if errors.Is(err, io.EOF) {
			if len(pending) != 0 {
				return errors.New("ai: I/O output must be valid UTF-8")
			}
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// ToolInterface receives AI-initiated tool calls.
type ToolInterface interface {
	Listen(context.Context) error
}

type ToolHandler func(context.Context, schemas.Message) (schemas.Message, error)

// ToolServer listens for AI tool-call messages on ToolSocketPath.
type ToolServer struct {
	Path    string
	Handler ToolHandler
}

func NewToolServer(handler ToolHandler) *ToolServer {
	return &ToolServer{Path: ToolSocketPath, Handler: handler}
}

func (s *ToolServer) Listen(ctx context.Context) error {
	if err := removeSocket(s.Path); err != nil {
		return err
	}
	listener, err := net.Listen("unix", s.Path)
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(s.Path)
	}()
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		go s.handle(ctx, conn)
	}
}

func (s *ToolServer) handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	payload, err := io.ReadAll(conn)
	if err != nil {
		return
	}
	var request schemas.Message
	if err := msgpack.Unmarshal(payload, &request); err != nil {
		return
	}
	response := schemas.Message{Type: "tool-result"}
	if s.Handler == nil {
		response.Response.Error = "tool server handler is unavailable"
	} else if response, err = s.Handler(ctx, request); err != nil {
		response = schemas.Message{Type: "tool-result", Response: schemas.McpCallResult{Error: err.Error()}}
	}
	encoded, err := msgpack.Marshal(response)
	if err == nil {
		_, _ = conn.Write(encoded)
	}
}

func removeSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("ai: refusing to replace non-socket %q", path)
	}
	return os.Remove(path)
}
