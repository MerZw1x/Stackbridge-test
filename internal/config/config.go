package config

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

const (
	StoragePostgres = "postgres"
	StorageLocal    = "local"
)

type Config struct {
	DBUser     string `env:"DATABASE_USER" env-default:""`
	DBHost     string `env:"DATABASE_HOST" env-default:""`
	DBName     string `env:"DATABASE_NAME" env-default:""`
	DBPassword string `env:"DATABASE_PASSWORD" env-default:""`
	DBPort     int    `env:"DATABASE_PORT" env-default:"5432"`

	ServerPort int `env:"SERVER_PORT" env-required:"true"`

	Storage string `env:"STORAGE" env-required:"true"`

	LogLevel string `env:"LOG_LEVEL" env-default:"info"`
}

func Load() (*Config, error) {
	cfg := &Config{}

	err := cleanenv.ReadEnv(cfg)
	if err != nil {
		return nil, err
	}

	switch cfg.Storage {
	case StoragePostgres, StorageLocal:
	default:
		return nil, fmt.Errorf("invalid STORAGE %q: must be %q or %q", cfg.Storage, StoragePostgres, StorageLocal)
	}

	if _, err := parseLogLevel(cfg.LogLevel); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (cfg *Config) GetDBDSN() string {
	return "host=" + cfg.DBHost +
		" port=" + strconv.Itoa(cfg.DBPort) +
		" user=" + cfg.DBUser +
		" password=" + cfg.DBPassword +
		" dbname=" + cfg.DBName +
		" sslmode=disable"
}

// GetLogLevel возвращает уровень логирования, валидность проверена в Load.
func (cfg *Config) GetLogLevel() slog.Level {
	level, _ := parseLogLevel(cfg.LogLevel)
	return level
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid LOG_LEVEL %q: must be debug, info, warn or error", raw)
	}
}
