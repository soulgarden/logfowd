package conf

import (
	"os"

	"github.com/jinzhu/configor"
)

type Config struct {
	Env       string   `json:"env" default:"prod"`
	DebugMode bool     `json:"debug_mode"  default:"false"`
	ES        *ES      `json:"elasticsearch"`
	LogsPath  []string `json:"logs_path" default:"/var/lib/docker/containers"`
}

type ES struct {
	Host          string `json:"host" default:"elasticsearch"`
	Port          string `json:"port" default:"9200"`
	IndexName     string `json:"index_name" default:"logfowd"`
	FlushInterval int    `json:"flush_interval" default:"1000"`
	Workers       int    `json:"workers" default:"10"`
}

func New() (*Config, error) {
	c := &Config{}
	path := os.Getenv("CFG_PATH")

	if path == "" {
		path = "./conf/config.json"
	}

	if err := configor.New(&configor.Config{ErrorOnUnmatchedKeys: true}).Load(c, path); err != nil {
		return nil, err
	}

	return c, nil
}
