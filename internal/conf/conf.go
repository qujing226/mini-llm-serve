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
	Address      string `koanf:"address"`
	AdminPort    uint64 `koanf:"adminPort"`
	ScheduleConf ScheduleConf
}

type ScheduleConf struct {
	QueueConf          QueueConf
	BatchRoundDuration uint64 `koanf:"batchRoundDuration"`

	MaxBatchSeq               uint64 `koanf:"maxBatchSeqs"`
	MaxBatchTokens            uint64 `koanf:"maxBatchTokens"`
	LongPrefillTokenThreshold uint64 `koanf:"longPrefillTokenThreshold"`
	MaxPartialPrefills        uint64 `koanf:"maxPartialPrefills"`
	MaxLongPartialPrefills    uint64 `koanf:"maxLongPartialPrefills"`
}

type QueueConf struct {
	QueueLength    uint64 `koanf:"queueLength"`
	QueueRoundTime uint64 `koanf:"queueRoundTime"`
}

func (s QueueConf) QueueRoundInterval() time.Duration {
	return time.Duration(s.QueueRoundTime) * time.Millisecond
}

func (s ScheduleConf) BatchRoundInterval() time.Duration {
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
