package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// SyncState is persisted between runs of vaultd.
type SyncState struct {
	LastSync    time.Time `json:"lastSync"`    // last successful sync
	LastAttempt time.Time `json:"lastAttempt"` // last attempt, ok or not
	LastOK      bool      `json:"lastOk"`
	LastError   string    `json:"lastError,omitempty"`
	LastChanged int       `json:"lastChanged"` // files downloaded in the last sync
	Running     bool      `json:"running"`
}

// Syncer runs the read-only Obsidian Sync client (through the "ob" wrapper).
type Syncer struct {
	mu        sync.Mutex
	state     SyncState
	statePath string
	command   []string // e.g. ["/bin/sh", "/mnt/us/documents/vault-app/ob", "sync"]
	wifiCmd   []string // e.g. ["lipc-get-prop", "com.lab126.wifid", "cmState"]; empty = always online
	onDone    func()
	busy      bool // an account operation (login/logout) is running
}

func NewSyncer(statePath string, command, wifiCmd []string, onDone func()) *Syncer {
	s := &Syncer{statePath: statePath, command: command, wifiCmd: wifiCmd, onDone: onDone}
	if data, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(data, &s.state)
	}
	s.state.Running = false
	return s
}

func (s *Syncer) State() SyncState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Syncer) Online() bool {
	if len(s.wifiCmd) == 0 {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, s.wifiCmd[0], s.wifiCmd[1:]...).Output()
	return err == nil && strings.TrimSpace(string(out)) == "CONNECTED"
}

// Start launches a sync in the background unless one is running.
// It returns false if a sync was already running.
func (s *Syncer) Start() bool {
	s.mu.Lock()
	if s.state.Running || s.busy {
		s.mu.Unlock()
		return false
	}
	s.state.Running = true
	s.state.LastAttempt = time.Now()
	s.mu.Unlock()
	go s.run()
	return true
}

// Exclusive runs fn while no sync can start, so login/logout never race a sync
// over the client's credentials. It returns false (fn not run) if a sync or
// another account operation is running.
func (s *Syncer) Exclusive(fn func()) bool {
	s.mu.Lock()
	if s.state.Running || s.busy {
		s.mu.Unlock()
		return false
	}
	s.busy = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.busy = false
		s.mu.Unlock()
	}()
	fn()
	return true
}

// MaybeStart syncs if online and the last attempt is older than maxAge.
func (s *Syncer) MaybeStart(maxAge time.Duration) {
	st := s.State()
	if st.Running || time.Since(st.LastAttempt) < maxAge {
		return
	}
	if s.Online() {
		s.Start()
	}
}

func (s *Syncer) run() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.command[0], s.command[1:]...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	text := out.String()

	s.mu.Lock()
	s.state.Running = false
	s.state.LastChanged = strings.Count(text, "downloaded file") + strings.Count(text, "deleted local file")
	if err != nil || strings.Contains(text, "STOP:") { // "STOP:" is the wrapper refusing to sync
		s.state.LastOK = false
		s.state.LastError = lastLines(text, 3)
		if s.state.LastError == "" && err != nil {
			s.state.LastError = err.Error()
		}
	} else {
		s.state.LastOK = true
		s.state.LastError = ""
		s.state.LastSync = time.Now()
	}
	data, _ := json.MarshalIndent(s.state, "", "  ")
	s.mu.Unlock()
	_ = os.WriteFile(s.statePath, data, 0o644)
	if s.onDone != nil {
		s.onDone()
	}
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
