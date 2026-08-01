package network

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/omurilo/canary-go/internal/transport"
)

// ProtocolFactory builds a fresh Protocol for each accepted connection.
type ProtocolFactory func() Protocol

// Service listens on a TCP port and dispatches connections to a protocol.
type Service struct {
	name       string
	addr       string
	serverName string
	factory    ProtocolFactory
	log        *slog.Logger
	ln         net.Listener
}

// NewService creates a listener service (not yet started). serverName is the
// configured world name; real clients prefix the game connection with
// "<serverName>\n" as a proxy identification handshake.
func NewService(name, addr, serverName string, factory ProtocolFactory, log *slog.Logger) *Service {
	return &Service{name: name, addr: addr, serverName: serverName, factory: factory, log: log}
}

// Start begins accepting connections until ctx is cancelled.
func (s *Service) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen %s (%s): %w", s.addr, s.name, err)
	}
	s.ln = ln
	s.log.Info("service listening", "service", s.name, "addr", s.addr)

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				s.log.Warn("accept error", "service", s.name, "err", err)
				continue
			}
		}
		c := &Connection{
			conn:       conn,
			codec:      transport.New(),
			proto:      s.factory(),
			serverName: s.serverName,
			log:        s.log.With("service", s.name, "peer", conn.RemoteAddr().String()),
		}
		go c.serve()
	}
}
