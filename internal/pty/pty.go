package pty

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	creackpty "github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// Process wraps a running PTY process.
type Process struct {
	ID  string
	ptm *os.File  // PTY master fd
	cmd *exec.Cmd
}

// Manager holds all active PTY processes keyed by a stable string ID.
// Processes survive tab switches — they are only removed when killed or
// when the underlying process exits naturally.
type Manager struct {
	mu    sync.Mutex
	procs map[string]*Process // key → process (key is also the process ID)
}

// NewManager returns an initialised Manager.
func NewManager() *Manager {
	return &Manager{procs: make(map[string]*Process)}
}

// Spawn starts a PTY process for key in workDir running argv.
// If a live process already exists for this key, its ID is returned
// unchanged (deduplication — same terminal survives tab switches).
func (m *Manager) Spawn(key, workDir string, argv []string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.procs[key]; ok {
		// process is still alive (reaper removes it from the map on exit)
		return key, nil
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptm, err := creackpty.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("pty start: %w", err)
	}

	proc := &Process{ID: key, ptm: ptm, cmd: cmd}
	m.procs[key] = proc

	// reap when process exits naturally
	go func() {
		cmd.Wait() //nolint:errcheck
		m.mu.Lock()
		delete(m.procs, key)
		m.mu.Unlock()
	}()

	return key, nil
}

// IsAlive reports whether a process with the given ID is running.
func (m *Manager) IsAlive(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.procs[id]
	return ok
}

// Kill terminates the process and removes it from the registry.
func (m *Manager) Kill(id string) error {
	m.mu.Lock()
	p, ok := m.procs[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("process %s not found", id)
	}
	delete(m.procs, id)
	m.mu.Unlock()

	if p.cmd.Process != nil {
		p.cmd.Process.Kill() //nolint:errcheck
	}
	return p.ptm.Close()
}

// ── WebSocket bridge ──────────────────────────────────────────────────────────

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ServeWS upgrades the HTTP connection to WebSocket and bidirectionally
// bridges it to the PTY identified by processID. Blocks until closed.
func (m *Manager) ServeWS(w http.ResponseWriter, r *http.Request, processID string) {
	m.mu.Lock()
	p, ok := m.procs[processID]
	m.mu.Unlock()
	if !ok {
		http.Error(w, "process not found", http.StatusNotFound)
		return
	}

	// Ensure the PTY reader goroutine exits when this handler returns.
	defer p.ptm.SetReadDeadline(time.Now()) //nolint:errcheck

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// PTY → WebSocket (goroutine)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := p.ptm.Read(buf)
			if err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
		}
	}()

	// WebSocket → PTY stdin (main loop)
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if _, err := p.ptm.Write(msg); err != nil {
			return
		}
	}
}
