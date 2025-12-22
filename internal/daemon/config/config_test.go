// Package config provides daemon-specific configuration.
package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	t.Run("listen config defaults", func(t *testing.T) {
		if cfg.Daemon.Listen.SocketPath == "" {
			t.Error("SocketPath should have default value")
		}
		if cfg.Daemon.Listen.AllowRemote {
			t.Error("AllowRemote should be false by default")
		}
	})

	t.Run("log config defaults", func(t *testing.T) {
		if cfg.Daemon.DaemonLog.Level != "info" {
			t.Errorf("Log.Level = %v, want info", cfg.Daemon.DaemonLog.Level)
		}
		if cfg.Daemon.DaemonLog.Format != "text" {
			t.Errorf("Log.Format = %v, want text", cfg.Daemon.DaemonLog.Format)
		}
	})

	t.Run("concurrency defaults", func(t *testing.T) {
		if cfg.Daemon.MaxConcurrentRuns != 10 {
			t.Errorf("MaxConcurrentRuns = %v, want 10", cfg.Daemon.MaxConcurrentRuns)
		}
	})

	t.Run("timeout defaults", func(t *testing.T) {
		if cfg.Daemon.DefaultTimeout != 30*time.Minute {
			t.Errorf("DefaultTimeout = %v, want 30m", cfg.Daemon.DefaultTimeout)
		}
		if cfg.Daemon.ShutdownTimeout != 30*time.Second {
			t.Errorf("ShutdownTimeout = %v, want 30s", cfg.Daemon.ShutdownTimeout)
		}
	})

	t.Run("checkpoints enabled by default", func(t *testing.T) {
		if !cfg.Daemon.CheckpointsEnabled {
			t.Error("CheckpointsEnabled should be true by default")
		}
	})

	t.Run("backend defaults", func(t *testing.T) {
		if cfg.Daemon.Backend.Type != "memory" {
			t.Errorf("Backend.Type = %v, want memory", cfg.Daemon.Backend.Type)
		}
	})

	t.Run("distributed defaults", func(t *testing.T) {
		if cfg.Daemon.Distributed.Enabled {
			t.Error("Distributed.Enabled should be false by default")
		}
		if !cfg.Daemon.Distributed.LeaderElection {
			t.Error("LeaderElection should be true by default")
		}
		if cfg.Daemon.Distributed.StalledJobTimeoutSeconds != 300 {
			t.Errorf("StalledJobTimeoutSeconds = %v, want 300", cfg.Daemon.Distributed.StalledJobTimeoutSeconds)
		}
	})
}

func TestLoad(t *testing.T) {
	// Save and restore environment
	savedSocket := os.Getenv("CONDUCTOR_SOCKET")
	savedTCPAddr := os.Getenv("CONDUCTOR_TCP_ADDR")
	savedPIDFile := os.Getenv("CONDUCTOR_PID_FILE")
	savedLogLevel := os.Getenv("CONDUCTOR_LOG_LEVEL")

	defer func() {
		os.Setenv("CONDUCTOR_SOCKET", savedSocket)
		os.Setenv("CONDUCTOR_TCP_ADDR", savedTCPAddr)
		os.Setenv("CONDUCTOR_PID_FILE", savedPIDFile)
		os.Setenv("CONDUCTOR_LOG_LEVEL", savedLogLevel)
	}()

	t.Run("default values when env not set", func(t *testing.T) {
		os.Unsetenv("CONDUCTOR_LISTEN_SOCKET")
		os.Unsetenv("CONDUCTOR_TCP_ADDR")
		os.Unsetenv("CONDUCTOR_PID_FILE")
		os.Unsetenv("CONDUCTOR_DAEMON_LOG_LEVEL")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		// Should get defaults
		if cfg.Daemon.Listen.SocketPath == "" {
			t.Error("SocketPath should have default value")
		}
		if cfg.Daemon.Listen.TCPAddr != "" {
			t.Error("TCPAddr should be empty by default")
		}
	})

	t.Run("env overrides socket", func(t *testing.T) {
		os.Setenv("CONDUCTOR_LISTEN_SOCKET", "/custom/socket.sock")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.Daemon.Listen.SocketPath != "/custom/socket.sock" {
			t.Errorf("SocketPath = %v, want /custom/socket.sock", cfg.Daemon.Listen.SocketPath)
		}
	})

	t.Run("env overrides tcp addr", func(t *testing.T) {
		os.Setenv("CONDUCTOR_TCP_ADDR", ":9000")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.Daemon.Listen.TCPAddr != ":9000" {
			t.Errorf("TCPAddr = %v, want :9000", cfg.Daemon.Listen.TCPAddr)
		}
	})

	t.Run("env overrides pid file", func(t *testing.T) {
		os.Setenv("CONDUCTOR_PID_FILE", "/var/run/conductor.pid")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.Daemon.PIDFile != "/var/run/conductor.pid" {
			t.Errorf("PIDFile = %v, want /var/run/conductor.pid", cfg.Daemon.PIDFile)
		}
	})

	t.Run("env overrides log level", func(t *testing.T) {
		os.Setenv("CONDUCTOR_DAEMON_LOG_LEVEL", "debug")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.Daemon.DaemonLog.Level != "debug" {
			t.Errorf("Log.Level = %v, want debug", cfg.Daemon.DaemonLog.Level)
		}
	})
}

func TestConfig_CheckpointDir(t *testing.T) {
	t.Run("checkpoints enabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Daemon.DataDir = "/var/lib/conductor"
		cfg.Daemon.CheckpointsEnabled = true

		dir := cfg.Daemon.CheckpointDir()
		expected := "/var/lib/conductor/checkpoints"
		if dir != expected {
			t.Errorf("CheckpointDir() = %v, want %v", dir, expected)
		}
	})

	t.Run("checkpoints disabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Daemon.DataDir = "/var/lib/conductor"
		cfg.Daemon.CheckpointsEnabled = false

		dir := cfg.Daemon.CheckpointDir()
		if dir != "" {
			t.Errorf("CheckpointDir() = %v, want empty string", dir)
		}
	})
}

func TestListenConfig(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		cfg := ListenConfig{}
		if cfg.AllowRemote {
			t.Error("AllowRemote should be false by default")
		}
	})

	t.Run("with TLS config", func(t *testing.T) {
		cfg := ListenConfig{
			TCPAddr: ":443",
			TLSCert: "/etc/ssl/cert.pem",
			TLSKey:  "/etc/ssl/key.pem",
		}

		if cfg.TLSCert == "" || cfg.TLSKey == "" {
			t.Error("TLS config should be set")
		}
	})
}

func TestAuthConfig(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		cfg := AuthConfig{}
		if cfg.Enabled {
			t.Error("Enabled should be false by default")
		}
		if cfg.AllowUnixSocket {
			t.Error("AllowUnixSocket should be false by default")
		}
	})

	t.Run("with API keys", func(t *testing.T) {
		cfg := AuthConfig{
			Enabled: true,
			APIKeys: []string{"key1", "key2"},
		}

		if len(cfg.APIKeys) != 2 {
			t.Errorf("APIKeys length = %v, want 2", len(cfg.APIKeys))
		}
	})
}

func TestBackendConfig(t *testing.T) {
	t.Run("memory backend", func(t *testing.T) {
		cfg := BackendConfig{
			Type: "memory",
		}

		if cfg.Type != "memory" {
			t.Errorf("Type = %v, want memory", cfg.Type)
		}
	})

	t.Run("postgres backend", func(t *testing.T) {
		cfg := BackendConfig{
			Type: "postgres",
			Postgres: PostgresConfig{
				ConnectionString:       "postgres://localhost/conductor",
				MaxOpenConns:           25,
				MaxIdleConns:           5,
				ConnMaxLifetimeSeconds: 300,
			},
		}

		if cfg.Postgres.MaxOpenConns != 25 {
			t.Errorf("MaxOpenConns = %v, want 25", cfg.Postgres.MaxOpenConns)
		}
	})
}

func TestDistributedConfig(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		cfg := DistributedConfig{}
		if cfg.Enabled {
			t.Error("Enabled should be false by default")
		}
		if cfg.LeaderElection {
			t.Error("LeaderElection should be false by default (zero value)")
		}
	})

	t.Run("with instance ID", func(t *testing.T) {
		cfg := DistributedConfig{
			Enabled:    true,
			InstanceID: "node-1",
		}

		if cfg.InstanceID != "node-1" {
			t.Errorf("InstanceID = %v, want node-1", cfg.InstanceID)
		}
	})
}

func TestWebhooksConfig(t *testing.T) {
	t.Run("empty routes", func(t *testing.T) {
		cfg := WebhooksConfig{}
		if len(cfg.Routes) != 0 {
			t.Error("Routes should be empty by default")
		}
	})

	t.Run("with routes", func(t *testing.T) {
		cfg := WebhooksConfig{
			Routes: []WebhookRoute{
				{
					Path:     "/webhooks/github",
					Source:   "github",
					Workflow: "github-handler",
					Events:   []string{"push", "pull_request"},
					Secret:   "${GITHUB_SECRET}",
				},
			},
		}

		if len(cfg.Routes) != 1 {
			t.Errorf("Routes length = %v, want 1", len(cfg.Routes))
		}
		if cfg.Routes[0].Path != "/webhooks/github" {
			t.Errorf("Path = %v, want /webhooks/github", cfg.Routes[0].Path)
		}
	})
}

func TestSchedulesConfig(t *testing.T) {
	t.Run("empty schedules", func(t *testing.T) {
		cfg := SchedulesConfig{}
		if cfg.Enabled {
			t.Error("Enabled should be false by default")
		}
		if len(cfg.Schedules) != 0 {
			t.Error("Schedules should be empty by default")
		}
	})

	t.Run("with schedules", func(t *testing.T) {
		cfg := SchedulesConfig{
			Enabled: true,
			Schedules: []ScheduleEntry{
				{
					Name:     "daily-backup",
					Cron:     "0 0 * * *",
					Workflow: "backup-workflow",
					Inputs:   map[string]any{"target": "database"},
					Enabled:  true,
					Timezone: "UTC",
				},
			},
		}

		if len(cfg.Schedules) != 1 {
			t.Errorf("Schedules length = %v, want 1", len(cfg.Schedules))
		}
		if cfg.Schedules[0].Cron != "0 0 * * *" {
			t.Errorf("Cron = %v, want 0 0 * * *", cfg.Schedules[0].Cron)
		}
	})
}
