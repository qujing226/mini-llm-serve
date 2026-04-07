package conf

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Conf struct {
	Server    ServerConf     `koanf:"server"`
	Executors []ExecutorConf `koanf:"executors"`
}

type ServerConf struct {
	Address            string `koanf:"address"`
	AdminPort          uint64 `koanf:"adminPort"`
	QueueRoundTime     uint64  `koanf:"queueRoundTime"`
	QueueLength        uint64 `koanf:"queueLength"`
	BatchSize          uint64 `koanf:"batchSize"`
	BatchRoundDuration uint64 `koanf:"batchRoundDuration"`
}

func (s ServerConf) QueueRoundInterval() time.Duration {
	return time.Duration(s.QueueRoundTime) * time.Millisecond
}

func (s ServerConf) BatchRoundInterval() time.Duration {
	return time.Duration(s.BatchRoundDuration) * time.Millisecond
}

type ExecutorConf struct {
	ID      string   `koanf:"id"`
	Kind    string   `koanf:"kind"`
	Address []string `koanf:"address"`
	Enabled bool     `koanf:"enabled"`
}

func NewConfFromPath(path string) (*Conf, error) {
	ext := filepath.Ext(path)
	var parser koanf.Parser
	switch ext {
	case ".toml":
		parser = toml.Parser()
	case ".yaml", ".yml":
		parser = yaml.Parser()
	case ".json":
		parser = json.Parser()
	default:
		return nil, fmt.Errorf("conf: unsupported file type: %s", ext)
	}
	k := koanf.New(".")
	if err := k.Load(file.Provider(path), parser); err != nil {
		return nil, err
	}

	var cfg Conf
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
