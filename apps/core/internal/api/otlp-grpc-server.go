package api

import (
	"context"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip" // registers gzip, which OpenTelemetry exporters send by default
	"google.golang.org/grpc/keepalive"
)

// Limits of the OTLP/gRPC receiver, so a client that misbehaves cannot hold unbounded resources.
const (
	// otlpGRPCMaxConnections caps the open connections of the receiver; one more waits to be accepted.
	otlpGRPCMaxConnections = 1024
	// otlpGRPCMaxStreams caps the requests in flight on one connection.
	otlpGRPCMaxStreams = 64
	// otlpGRPCMaxHeaderBytes caps the request headers (the ingest key is the only one that matters).
	otlpGRPCMaxHeaderBytes = 32 << 10
	// otlpGRPCMaxSendBytes caps a response; an Export response is a few bytes plus a short error message.
	otlpGRPCMaxSendBytes = 64 << 10
	// otlpGRPCHandshakeTimeout is how long a new connection may take to finish its HTTP/2 setup.
	otlpGRPCHandshakeTimeout = 10 * time.Second
)

// otlpGRPCShutdownGrace is how long GracefulStop may take before the remaining connections are cut.
var otlpGRPCShutdownGrace = 10 * time.Second

// otlpGRPCMaxMessageBytes is the largest request the receiver reads, after decompression: the cap OTLP/HTTP
// puts on its body (gzip included).
var otlpGRPCMaxMessageBytes = int(maxBodyBytesFor("/v1/ingest/otlp/", ""))

// newOTLPGRPCServer builds the gRPC server with the OTLP logs, metrics and trace services registered.
// Calls are authenticated by the ingest key in the "authorization" metadata before their message is read.
func (s *Server) newOTLPGRPCServer() *grpc.Server {
	srv := grpc.NewServer(
		grpc.ForceServerCodecV2(newOTLPWireCodec()),
		grpc.MaxRecvMsgSize(otlpGRPCMaxMessageBytes),
		grpc.MaxSendMsgSize(otlpGRPCMaxSendBytes),
		grpc.MaxHeaderListSize(otlpGRPCMaxHeaderBytes),
		grpc.MaxConcurrentStreams(otlpGRPCMaxStreams),
		grpc.ConnectionTimeout(otlpGRPCHandshakeTimeout),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     5 * time.Minute,  // an idle exporter reconnects when it next sends
			MaxConnectionAge:      30 * time.Minute, // rebalances behind a proxy; clients reconnect on the GOAWAY
			MaxConnectionAgeGrace: 30 * time.Second,
			Time:                  2 * time.Minute, // ping a silent client, and drop it when it does not answer
			Timeout:               20 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{MinTime: 10 * time.Second}),
		grpc.InTapHandle(s.authenticateOTLPGRPC),
		grpc.UnaryInterceptor(s.recoverOTLPGRPC),
	)
	s.registerOTLPGRPCServices(srv)
	return srv
}

// StartOTLPGRPC starts the receiver on addr in the background and returns the address it listens on and a
// channel that closes once it has stopped, which it does when ctx is cancelled. An empty addr leaves the
// receiver off: it returns nil values and no error. A failure to listen is returned to the caller.
func (s *Server) StartOTLPGRPC(ctx context.Context, addr string) (net.Addr, <-chan struct{}, error) {
	if addr == "" {
		return nil, nil, nil
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := s.ServeOTLPGRPC(ctx, listener); err != nil {
			s.logger.Error("OTLP/gRPC receiver stopped", "error", err)
		}
	}()
	return listener.Addr(), done, nil
}

// ServeOTLPGRPC serves OTLP/gRPC on listener until ctx is cancelled, then stops gracefully: in-flight
// exports finish, and after otlpGRPCShutdownGrace the remaining connections are cut. It returns the
// serve error, or nil after a clean stop.
func (s *Server) ServeOTLPGRPC(ctx context.Context, listener net.Listener) error {
	srv := s.newOTLPGRPCServer()
	s.logger.Info("Starting OTLP/gRPC receiver", "address", listener.Addr().String())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(newLimitListener(listener, otlpGRPCMaxConnections)) }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.logger.Info("Shutting down OTLP/gRPC receiver")
		stopped := make(chan struct{})
		go func() {
			srv.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(otlpGRPCShutdownGrace):
			s.logger.Warn("OTLP/gRPC receiver did not drain within the grace period; closing remaining connections")
			srv.Stop()
			<-stopped
		}
		return <-errCh
	}
}

// limitListener holds Accept while max connections are open, so the number of connections a flood of
// clients can keep is bounded; the others wait in the kernel's backlog.
type limitListener struct {
	net.Listener
	slots     chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

func newLimitListener(l net.Listener, max int) *limitListener {
	return &limitListener{Listener: l, slots: make(chan struct{}, max), done: make(chan struct{})}
}

func (l *limitListener) Accept() (net.Conn, error) {
	select {
	case l.slots <- struct{}{}:
	case <-l.done:
		return nil, net.ErrClosed
	}
	conn, err := l.Listener.Accept()
	if err != nil {
		<-l.slots
		return nil, err
	}
	return &limitConn{Conn: conn, release: func() { <-l.slots }}, nil
}

// Close also wakes an Accept that is waiting for a free slot, so a stopping server never hangs on it.
func (l *limitListener) Close() error {
	l.closeOnce.Do(func() { close(l.done) })
	return l.Listener.Close()
}

type limitConn struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *limitConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(c.release)
	return err
}
