// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package debug provides debug session management for interactive workflow debugging.
package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/tombee/conductor/internal/tracing/storage"
)

// SessionState represents the current state of a debug session.
type SessionState string

const (
	// SessionStateInitialized indicates the session has been created but not yet started.
	SessionStateInitialized SessionState = "INITIALIZED"
	// SessionStateRunning indicates the workflow is actively executing.
	SessionStateRunning SessionState = "RUNNING"
	// SessionStatePaused indicates the workflow is paused at a breakpoint.
	SessionStatePaused SessionState = "PAUSED"
	// SessionStateCompleted indicates the workflow has completed successfully.
	SessionStateCompleted SessionState = "COMPLETED"
	// SessionStateFailed indicates the workflow has failed.
	SessionStateFailed SessionState = "FAILED"
	// SessionStateTimeout indicates the session has exceeded its timeout.
	SessionStateTimeout SessionState = "TIMEOUT"
	// SessionStateKilled indicates the session was manually terminated.
	SessionStateKilled SessionState = "KILLED"
)

// DebugSession represents an interactive debugging session.
type DebugSession struct {
	SessionID     string       `json:"session_id"`
	RunID         string       `json:"run_id"`
	CurrentStepID string       `json:"current_step_id,omitempty"`
	State         SessionState `json:"state"`
	Breakpoints   []string     `json:"breakpoints,omitempty"`
	EventBuffer   []Event      `json:"-"` // Not persisted directly
	LastActivity  time.Time    `json:"last_activity"`
	CreatedAt     time.Time    `json:"created_at"`
	ExpiresAt     time.Time    `json:"expires_at"`

	// observers tracks connected clients (session owner + read-only observers)
	observers map[string]*Observer
	mu        sync.RWMutex
}

// Observer represents a client connected to a debug session.
type Observer struct {
	ID       string
	IsOwner  bool
	JoinedAt time.Time
}

// SessionManager manages debug sessions with state machine transitions and persistence.
type SessionManager struct {
	store           *storage.SQLiteStore
	sessionTimeout  time.Duration
	maxEventBuffer  int
	maxObservers    int
	sessions        map[string]*DebugSession
	mu              sync.RWMutex
	commandChannels map[string]chan Command
}

// Command represents a debug command from a client.
type Command struct {
	Type    CommandType    `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

// CommandType represents the type of debug command.
type CommandType string

const (
	CommandContinue CommandType = "continue"
	CommandNext     CommandType = "next"
	CommandSkip     CommandType = "skip"
	CommandAbort    CommandType = "abort"
	CommandInspect  CommandType = "inspect"
	CommandContext  CommandType = "context"
)

// SessionManagerConfig configures the session manager.
type SessionManagerConfig struct {
	Store          *storage.SQLiteStore
	SessionTimeout time.Duration
	MaxEventBuffer int
	MaxObservers   int
}

// NewSessionManager creates a new debug session manager.
func NewSessionManager(cfg SessionManagerConfig) *SessionManager {
	if cfg.SessionTimeout == 0 {
		cfg.SessionTimeout = 60 * time.Minute
	}
	if cfg.MaxEventBuffer == 0 {
		cfg.MaxEventBuffer = 100
	}
	if cfg.MaxObservers == 0 {
		cfg.MaxObservers = 10
	}

	return &SessionManager{
		store:           cfg.Store,
		sessionTimeout:  cfg.SessionTimeout,
		maxEventBuffer:  cfg.MaxEventBuffer,
		maxObservers:    cfg.MaxObservers,
		sessions:        make(map[string]*DebugSession),
		commandChannels: make(map[string]chan Command),
	}
}

// CreateSession creates a new debug session.
func (m *SessionManager) CreateSession(ctx context.Context, runID string, breakpoints []string) (*DebugSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionID := fmt.Sprintf("debug-%s-%d", runID, time.Now().UnixNano())
	now := time.Now()
	expiresAt := now.Add(m.sessionTimeout)

	session := &DebugSession{
		SessionID:    sessionID,
		RunID:        runID,
		State:        SessionStateInitialized,
		Breakpoints:  breakpoints,
		EventBuffer:  make([]Event, 0, m.maxEventBuffer),
		LastActivity: now,
		CreatedAt:    now,
		ExpiresAt:    expiresAt,
		observers:    make(map[string]*Observer),
	}

	// Persist to database
	if err := m.persistSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to persist session: %w", err)
	}

	// Add to in-memory cache
	m.sessions[sessionID] = session
	m.commandChannels[sessionID] = make(chan Command, 10)

	return session, nil
}

// GetSession retrieves a session by ID, loading from database if not in memory.
func (m *SessionManager) GetSession(ctx context.Context, sessionID string) (*DebugSession, error) {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if exists {
		return session, nil
	}

	// Load from database
	session, err := m.loadSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Add to cache
	m.mu.Lock()
	m.sessions[sessionID] = session
	if m.commandChannels[sessionID] == nil {
		m.commandChannels[sessionID] = make(chan Command, 10)
	}
	m.mu.Unlock()

	return session, nil
}

// UpdateSessionState updates the session state with proper state machine transitions.
func (m *SessionManager) UpdateSessionState(ctx context.Context, sessionID string, newState SessionState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Validate state transition
	if !m.isValidTransition(session.State, newState) {
		return fmt.Errorf("invalid state transition from %s to %s", session.State, newState)
	}

	session.State = newState
	session.LastActivity = time.Now()

	// Persist to database
	if err := m.persistSession(ctx, session); err != nil {
		return fmt.Errorf("failed to persist session state: %w", err)
	}

	return nil
}

// isValidTransition checks if a state transition is allowed.
func (m *SessionManager) isValidTransition(from, to SessionState) bool {
	// Define valid state transitions
	validTransitions := map[SessionState][]SessionState{
		SessionStateInitialized: {SessionStateRunning, SessionStateKilled},
		SessionStateRunning:     {SessionStatePaused, SessionStateCompleted, SessionStateFailed, SessionStateTimeout, SessionStateKilled},
		SessionStatePaused:      {SessionStateRunning, SessionStateCompleted, SessionStateFailed, SessionStateTimeout, SessionStateKilled},
		SessionStateCompleted:   {}, // Terminal state
		SessionStateFailed:      {}, // Terminal state
		SessionStateTimeout:     {}, // Terminal state
		SessionStateKilled:      {}, // Terminal state
	}

	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}

	for _, state := range allowed {
		if state == to {
			return true
		}
	}

	return false
}

// UpdateCurrentStep updates the current step ID for the session.
func (m *SessionManager) UpdateCurrentStep(ctx context.Context, sessionID, stepID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.CurrentStepID = stepID
	session.LastActivity = time.Now()

	// Persist to database
	if err := m.persistSession(ctx, session); err != nil {
		return fmt.Errorf("failed to persist session: %w", err)
	}

	return nil
}

// AddEvent adds an event to the session's event buffer.
func (m *SessionManager) AddEvent(sessionID string, event Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Add to buffer with size limit
	session.EventBuffer = append(session.EventBuffer, event)
	if len(session.EventBuffer) > m.maxEventBuffer {
		// Remove oldest events to maintain buffer size
		session.EventBuffer = session.EventBuffer[len(session.EventBuffer)-m.maxEventBuffer:]
	}

	session.LastActivity = time.Now()

	return nil
}

// GetEventBuffer retrieves the event buffer for a session.
func (m *SessionManager) GetEventBuffer(sessionID string) ([]Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Return a copy of the buffer
	buffer := make([]Event, len(session.EventBuffer))
	copy(buffer, session.EventBuffer)

	return buffer, nil
}

// AddObserver adds an observer to a session.
func (m *SessionManager) AddObserver(sessionID, observerID string, isOwner bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// Check observer limit
	if len(session.observers) >= m.maxObservers {
		return fmt.Errorf("maximum observers (%d) reached for session", m.maxObservers)
	}

	session.observers[observerID] = &Observer{
		ID:       observerID,
		IsOwner:  isOwner,
		JoinedAt: time.Now(),
	}

	return nil
}

// RemoveObserver removes an observer from a session.
func (m *SessionManager) RemoveObserver(sessionID, observerID string) error {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	delete(session.observers, observerID)

	return nil
}

// GetObserverCount returns the number of observers for a session.
func (m *SessionManager) GetObserverCount(sessionID string) (int, error) {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return 0, fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

	return len(session.observers), nil
}

// IsObserver checks if an observer is connected to a session.
func (m *SessionManager) IsObserver(sessionID, observerID string) (bool, bool) {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return false, false
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

	observer, found := session.observers[observerID]
	if !found {
		return false, false
	}

	return true, observer.IsOwner
}

// SendCommand sends a command to a session.
func (m *SessionManager) SendCommand(sessionID string, cmd Command) error {
	m.mu.RLock()
	ch, exists := m.commandChannels[sessionID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Non-blocking send
	select {
	case ch <- cmd:
		return nil
	default:
		return fmt.Errorf("command channel full for session %s", sessionID)
	}
}

// GetCommandChannel returns the command channel for a session.
func (m *SessionManager) GetCommandChannel(sessionID string) (<-chan Command, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ch, exists := m.commandChannels[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return ch, nil
}

// ListSessions returns all active sessions.
func (m *SessionManager) ListSessions(ctx context.Context) ([]*DebugSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*DebugSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// CleanupExpiredSessions removes sessions that have exceeded their timeout.
func (m *SessionManager) CleanupExpiredSessions(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	count := 0

	for sessionID, session := range m.sessions {
		if now.After(session.ExpiresAt) {
			// Mark as timed out
			session.State = SessionStateTimeout
			if err := m.persistSession(ctx, session); err != nil {
				// Log error but continue cleanup
				continue
			}

			// Remove from memory
			delete(m.sessions, sessionID)
			if ch, exists := m.commandChannels[sessionID]; exists {
				close(ch)
				delete(m.commandChannels, sessionID)
			}

			count++
		}
	}

	return count, nil
}

// CleanupCompletedSessions removes sessions that completed more than 24 hours ago.
// This is run periodically to reclaim storage space.
func (m *SessionManager) CleanupCompletedSessions(ctx context.Context) (int, error) {
	// Find completed sessions older than 24 hours
	cutoff := time.Now().Add(-24 * time.Hour)

	query := `
		DELETE FROM debug_sessions
		WHERE state IN (?, ?, ?, ?)
		AND created_at < ?
	`

	result, err := m.store.DB().ExecContext(ctx, query,
		string(SessionStateCompleted),
		string(SessionStateFailed),
		string(SessionStateTimeout),
		string(SessionStateKilled),
		cutoff.UnixNano(),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup completed sessions: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get cleanup count: %w", err)
	}

	// Also remove from memory cache if present
	m.mu.Lock()
	for sessionID, session := range m.sessions {
		if session.CreatedAt.Before(cutoff) &&
			(session.State == SessionStateCompleted ||
				session.State == SessionStateFailed ||
				session.State == SessionStateTimeout ||
				session.State == SessionStateKilled) {
			delete(m.sessions, sessionID)
			if ch, exists := m.commandChannels[sessionID]; exists {
				close(ch)
				delete(m.commandChannels, sessionID)
			}
		}
	}
	m.mu.Unlock()

	return int(count), nil
}

// DeleteSession deletes a session from memory and database.
func (m *SessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from memory
	delete(m.sessions, sessionID)
	if ch, exists := m.commandChannels[sessionID]; exists {
		close(ch)
		delete(m.commandChannels, sessionID)
	}

	// Delete from database
	query := "DELETE FROM debug_sessions WHERE session_id = ?"
	if _, err := m.store.DB().ExecContext(ctx, query, sessionID); err != nil {
		return fmt.Errorf("failed to delete session from database: %w", err)
	}

	return nil
}

// persistSession saves a session to the database.
func (m *SessionManager) persistSession(ctx context.Context, session *DebugSession) error {
	breakpointsJSON, err := json.Marshal(session.Breakpoints)
	if err != nil {
		return fmt.Errorf("failed to marshal breakpoints: %w", err)
	}

	eventBufferJSON, err := json.Marshal(session.EventBuffer)
	if err != nil {
		return fmt.Errorf("failed to marshal event buffer: %w", err)
	}

	query := `
		INSERT INTO debug_sessions (
			session_id, run_id, current_step_id, state, breakpoints,
			event_buffer, last_activity, created_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			current_step_id = excluded.current_step_id,
			state = excluded.state,
			breakpoints = excluded.breakpoints,
			event_buffer = excluded.event_buffer,
			last_activity = excluded.last_activity,
			expires_at = excluded.expires_at
	`

	_, err = m.store.DB().ExecContext(ctx, query,
		session.SessionID,
		session.RunID,
		session.CurrentStepID,
		string(session.State),
		string(breakpointsJSON),
		string(eventBufferJSON),
		session.LastActivity.UnixNano(),
		session.CreatedAt.UnixNano(),
		session.ExpiresAt.UnixNano(),
	)
	if err != nil {
		return fmt.Errorf("failed to persist session: %w", err)
	}

	return nil
}

// loadSession loads a session from the database.
func (m *SessionManager) loadSession(ctx context.Context, sessionID string) (*DebugSession, error) {
	query := `
		SELECT session_id, run_id, current_step_id, state, breakpoints,
			event_buffer, last_activity, created_at, expires_at
		FROM debug_sessions WHERE session_id = ?
	`

	var session DebugSession
	var breakpointsJSON, eventBufferJSON string
	var lastActivity, createdAt, expiresAt int64

	err := m.store.DB().QueryRowContext(ctx, query, sessionID).Scan(
		&session.SessionID,
		&session.RunID,
		&session.CurrentStepID,
		&session.State,
		&breakpointsJSON,
		&eventBufferJSON,
		&lastActivity,
		&createdAt,
		&expiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	// Parse breakpoints
	if err := json.Unmarshal([]byte(breakpointsJSON), &session.Breakpoints); err != nil {
		return nil, fmt.Errorf("failed to unmarshal breakpoints: %w", err)
	}

	// Parse event buffer
	if err := json.Unmarshal([]byte(eventBufferJSON), &session.EventBuffer); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event buffer: %w", err)
	}

	session.LastActivity = time.Unix(0, lastActivity)
	session.CreatedAt = time.Unix(0, createdAt)
	session.ExpiresAt = time.Unix(0, expiresAt)
	session.observers = make(map[string]*Observer)

	return &session, nil
}
