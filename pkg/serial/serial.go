// Package serial exposes the host interface through a local Unix socket.
package serial

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/ghofuobudgeghikhdrhiehdihebd/airlock/pkg/schemas"
)

const SocketPath = "/tmp/aerolog.sock"

// Session is one persistent, MessagePack-framed PC-driver connection.
// Send is safe to call from the serial request handler and the AI-tool
// handler, but only one caller may wait for Receive at a time.
type Session struct {
	conn    net.Conn
	decoder *msgpack.Decoder
	encoder *msgpack.Encoder
	writeMu sync.Mutex
}

func newSession(conn net.Conn) *Session {
	return &Session{conn: conn, decoder: msgpack.NewDecoder(conn), encoder: msgpack.NewEncoder(conn)}
}

func (s *Session) Receive() (schemas.Message, error) {
	var message schemas.Message
	err := s.decoder.Decode(&message)
	return message, err
}

func (s *Session) Send(message schemas.Message) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.encoder.Encode(message)
}

func (s *Session) Close() error { return s.conn.Close() }

// Handler owns one PC-driver session for the lifetime of its request.
type Handler func(context.Context, *Session) error

type Server struct {
	Path    string
	Handler Handler
}

func New() *Server {
	return &Server{Path: SocketPath}
}

func (s *Server) Listen(ctx context.Context) error {
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
		go s.handle(ctx, newSession(conn))
	}
}

func (s *Server) handle(ctx context.Context, session *Session) {
	defer session.Close()
	if s.Handler != nil {
		_ = s.Handler(ctx, session)
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
		return errors.New("serial: refusing to replace a non-socket path")
	}
	return os.Remove(path)
}
