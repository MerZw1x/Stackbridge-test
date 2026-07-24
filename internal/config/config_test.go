package config

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for key, value := range env {
		t.Setenv(key, value)
	}
}

func baseEnv() map[string]string {
	return map[string]string{
		"SERVER_PORT":       "8080",
		"STORAGE":           StorageLocal,
		"LOG_LEVEL":         "info",
		"DATABASE_HOST":     "localhost",
		"DATABASE_PORT":     "5432",
		"DATABASE_NAME":     "subscriptions",
		"DATABASE_USER":     "postgres",
		"DATABASE_PASSWORD": "postgres",
	}
}

func TestLoad_Success(t *testing.T) {
	setEnv(t, baseEnv())

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.ServerPort)
	assert.Equal(t, StorageLocal, cfg.Storage)
	assert.Equal(t, slog.LevelInfo, cfg.GetLogLevel())
}

func TestLoad_InvalidStorage(t *testing.T) {
	env := baseEnv()
	env["STORAGE"] = "mongo"
	setEnv(t, env)

	_, err := Load()
	assert.Error(t, err)
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	env := baseEnv()
	env["LOG_LEVEL"] = "verbose"
	setEnv(t, env)

	_, err := Load()
	assert.Error(t, err)
}

func TestLoad_MissingRequired(t *testing.T) {
	env := baseEnv()
	delete(env, "SERVER_PORT")
	setEnv(t, env)
	t.Setenv("SERVER_PORT", "")

	_, err := Load()
	assert.Error(t, err)
}

func TestGetLogLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"WARN":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"Error":   slog.LevelError,
	}

	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			assert.Equal(t, want, (&Config{LogLevel: raw}).GetLogLevel())
		})
	}
}

func TestGetDBDSN(t *testing.T) {
	cfg := &Config{
		DBHost:     "db",
		DBPort:     5432,
		DBUser:     "postgres",
		DBPassword: "secret",
		DBName:     "subscriptions",
	}

	assert.Equal(t,
		"host=db port=5432 user=postgres password=secret dbname=subscriptions sslmode=disable",
		cfg.GetDBDSN())
}
