package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

const (
	socketMode    = 0o660
	socketDirMode = 0o750
	// maxLogBytes caps how much of the log tail a single request returns.
	maxLogBytes = 4 << 20
)

// Server serves the control API on a unix socket.
type Server struct {
	path     string
	handler  Handler
	listener net.Listener
	http     *http.Server
}

// NewServer prepares a server; call Start to bind the socket.
func NewServer(path string, h Handler) *Server {
	s := &Server{path: path, handler: h}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/status", s.status)
	mux.HandleFunc("POST /v1/maintenance", s.maintenance)
	mux.HandleFunc("POST /v1/token", s.token)
	mux.HandleFunc("POST /v1/reload", s.reload)
	mux.HandleFunc("GET /v1/logs", s.logs)
	mux.HandleFunc("GET /v1/token-check", s.tokenCheck)
	mux.HandleFunc("GET /v1/spool", s.spool)
	s.http = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return s
}

// Path returns the socket path.
func (s *Server) Path() string { return s.path }

// Start binds the socket (replacing a stale one) and serves in the background.
func (s *Server) Start() error {
	if err := os.MkdirAll(filepath.Dir(s.path), socketDirMode); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}
	if err := removeStale(s.path); err != nil {
		return err
	}
	ln, err := net.Listen("unix", s.path)
	if err != nil {
		return fmt.Errorf("listen on control socket: %w", err)
	}
	if err := os.Chmod(s.path, socketMode); err != nil {
		ln.Close()
		return fmt.Errorf("set control socket mode: %w", err)
	}
	grantGroup(filepath.Dir(s.path), s.path)
	s.listener = ln
	go func() { _ = s.http.Serve(ln) }()
	return nil
}

// Close stops serving and removes the socket file.
func (s *Server) Close() error {
	err := s.http.Close()
	_ = os.Remove(s.path)
	return err
}

// removeStale deletes a leftover socket but refuses to replace a live daemon.
func removeStale(path string) error {
	if _, err := os.Lstat(path); err != nil {
		return nil
	}
	if conn, err := net.DialTimeout("unix", path, time.Second); err == nil {
		conn.Close()
		return fmt.Errorf("another agent is already listening on %s", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove stale control socket: %w", err)
	}
	return nil
}

// grantGroup hands the socket (and its directory) to the orion group when the
// group exists and we are allowed to; failure is not fatal because the daemon
// normally already runs as the orion user.
func grantGroup(paths ...string) {
	g, err := user.LookupGroup(Group)
	if err != nil && runtime.GOOS == "darwin" {
		// macOS has no orion group by default; local admins operate the agent.
		if g, err = user.LookupGroup("_orion"); err != nil {
			g, err = user.LookupGroup("admin")
		}
	}
	if err != nil {
		return
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return
	}
	for _, p := range paths {
		_ = os.Chown(p, -1, gid)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, Error{Message: err.Error()})
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	st, err := s.handler.Status()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) maintenance(w http.ResponseWriter, r *http.Request) {
	var req MaintenanceRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.handler.SetMaintenance(req); err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	var req TokenRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil || req.Token == "" {
		writeErr(w, http.StatusBadRequest, errors.New("token is required"))
		return
	}
	if err := s.handler.ApplyToken(req.Token); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) reload(w http.ResponseWriter, _ *http.Request) {
	if err := s.handler.Reload(); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// logs streams the tail of the daemon's JSONL log file so unprivileged
// operators can read logs the orion user owns.
func (s *Server) logs(w http.ResponseWriter, _ *http.Request) {
	path := s.handler.LogPath()
	if path == "" {
		writeErr(w, http.StatusNotFound, errors.New("file logging is not enabled"))
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.Size() > maxLogBytes {
		_, _ = f.Seek(info.Size()-maxLogBytes, io.SeekStart)
		// Drop the partial first line.
		buf := make([]byte, 1)
		for {
			if _, err := f.Read(buf); err != nil || buf[0] == '\n' {
				break
			}
		}
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	_, _ = io.Copy(w, f)
}

func (s *Server) tokenCheck(w http.ResponseWriter, _ *http.Request) {
	code, err := s.handler.TokenStatus()
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, TokenCheck{HTTPStatus: code})
}

func (s *Server) spool(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(r.URL.Query().Get("lines"))
	if n <= 0 || n > 5000 {
		n = 200
	}
	data, err := s.handler.SpoolRecent(n)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	_, _ = w.Write(data)
}
