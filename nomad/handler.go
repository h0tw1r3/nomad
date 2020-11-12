package nomad

import (
	"fmt"
	"net"
	"net/http"

	"google.golang.org/grpc"
)

type Handler struct {
	srv      *grpc.Server
	listener *chanListener
}

func NewHandler(addr net.Addr, register func(server *grpc.Server)) *Handler {
	// We don't need to pass tls.Config to the server since it's multiplexed
	// behind the RPC listener, which already has TLS configured.
	srv := grpc.NewServer()

	register(srv)

	lis := &chanListener{addr: addr, conns: make(chan net.Conn), done: make(chan struct{})}
	return &Handler{srv: srv, listener: lis}
}

// Handle the connection by sending it to a channel for the grpc.Server to receive.
func (h *Handler) Handle(conn net.Conn) {
	h.listener.conns <- conn
}

func (h *Handler) Run() error {
	return h.srv.Serve(h.listener)
}

func (h *Handler) Shutdown() error {
	h.srv.Stop()
	return nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.srv.ServeHTTP(w, r)
}

// chanListener implements net.Listener for grpc.Server.
type chanListener struct {
	conns chan net.Conn
	addr  net.Addr
	done  chan struct{}
}

// Accept blocks until a connection is received from Handle, and then returns the
// connection. Accept implements part of the net.Listener interface for grpc.Server.
func (l *chanListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		return nil, &net.OpError{
			Op:   "accept",
			Net:  l.addr.Network(),
			Addr: l.addr,
			Err:  fmt.Errorf("listener closed"),
		}
	}
}

func (l *chanListener) Addr() net.Addr {
	return l.addr
}

func (l *chanListener) Close() error {
	close(l.done)
	return nil
}

// NoOpHandler implements the same methods as Handler, but performs no handling.
// It may be used in place of Handler to disable the grpc server.
type NoOpHandler struct {
	Logger Logger
}

type Logger interface {
	Error(string, ...interface{})
}

func (h NoOpHandler) Handle(conn net.Conn) {
	h.Logger.Error("gRPC conn opened but gRPC RPC is disabled, closing",
		"conn", logConn(conn))
	_ = conn.Close()
}

func (h NoOpHandler) Run() error {
	return nil
}

func (h NoOpHandler) Shutdown() error {
	return nil
}

// logConn is a local copy of github.com/hashicorp/memberlist.LogConn, to avoid
// a large dependency for a minor formatting function.
// logConn is used to keep log formatting consistent.
func logConn(conn net.Conn) string {
	if conn == nil {
		return "from=<unknown address>"
	}
	addr := conn.RemoteAddr()
	if addr == nil {
		return "from=<unknown address>"
	}

	return fmt.Sprintf("from=%s", addr.String())
}