package config

import (
	"encoding/json"
	"os"

	"github.com/caarlos0/env/v6"

	"github.com/sergiusd/go-scanty-url-shortener/internal/model"
)

type Config struct {
	LogLevel string `json:"log_level" env:"SHORTENER_LOG_LEVEL"`
	Server   `json:"server"`
	Storage  `json:"storage"`
	Cache    `json:"cache"`
}

type Server struct {
	Port        string         `json:"port" env:"SHORTENER_SERVER_PORT"`
	Schema      string         `json:"schema" env:"SHORTENER_SERVER_SCHEMA"`
	Prefix      string         `json:"prefix" env:"SHORTENER_SERVER_PREFIX"`
	Err404      string         `json:"err404" env:"SHORTENER_SERVER_ERR404"`
	Token       string         `json:"token" env:"SHORTENER_SERVER_TOKEN"`
	ReadTimeout model.Duration `json:"readTimeout" env:"SHORTENER_SERVER_READ_TIMEOUT"`
	IdleTimeout model.Duration `json:"idleTimeout" env:"SHORTENER_SERVER_IDLE_TIMEOUT"`
}

type Storage struct {
	Kind  string `json:"kind" env:"SHORTENER_STORAGE_KIND"`
	Redis struct {
		Host     string `json:"host" env:"SHORTENER_REDIS_HOST"`
		Port     int    `json:"port" env:"SHORTENER_REDIS_PORT"`
		Password string `json:"password" env:"SHORTENER_REDIS_PASSWORD"`
	} `json:"redis"`
	Psql struct {
		Host     string         `json:"host" env:"SHORTENER_PSQL_HOST"`
		Port     int            `json:"port" env:"SHORTENER_PSQL_PORT"`
		User     string         `json:"user" env:"SHORTENER_PSQL_USER"`
		Password string         `json:"password" env:"SHORTENER_PSQL_PASSWORD"`
		Name     string         `json:"name" env:"SHORTENER_PSQL_NAME"`
		PoolSize int32          `json:"poolSize" env:"SHORTENER_PSQL_POOL_SIZE"`
		Timeout  model.Duration `json:"timeout" env:"SHORTENER_PSQL_TIMEOUT"`
	} `json:"psql"`
	Bolt struct {
		Path    string `json:"path" env:"SHORTENER_BOLT_PATH"`
		Bucket  string `json:"bucket" env:"SHORTENER_BOLT_BUCKET"`
		Timeout string `json:"timeout" env:"SHORTENER_BOLT_TIMEOUT"`
	} `json:"bolt"`
}

type Cache struct {
	Size int `json:"size" env:"SHORTENER_CACHE_SIZE"`
}

func FromFileAndEnv(mainPath string, extraPath ...string) (*Config, error) {
	var cfg Config

	allPath := []string{mainPath}
	allPath = append(allPath, extraPath...)

	for _, path := range allPath {
		if _, err := os.Stat(path); err == nil {
			b, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(b, &cfg); err != nil {
				return nil, err
			}
		}
	}

	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
