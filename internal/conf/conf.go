package conf

import (
	"fmt"
	"path/filepath"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Conf struct {
	Server ServerConf `koanf:"server"`
}

type ServerConf struct {
	Address        string `koanf:"address"`
	QueueRoundTime int64  `koanf:"queueRoundTime"`
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
