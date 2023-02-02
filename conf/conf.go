package conf

import (
	"os"

	"github.com/jinzhu/configor"
)

type Config struct {
	Env       string   `json:"env" default:"prod"`
	DebugMode bool     `json:"debug_mode"  default:"false"`
	Storage   Storage  `json:"storage"`
	LogsPath  []string `json:"logs_path" default:"/var/log/pods"`
}

type Storage struct {
	Host          string `json:"host" default:"elasticsearch"`
	Port          string `json:"port" default:"9200"`
	IndexName     string `json:"index_name" default:"logfowd"`
	FlushInterval int    `json:"flush_interval" default:"1000"`
	Workers       int    `json:"workers" default:"10"`
	APIPrefix     string `json:"api_prefix" default:""`
	UseAuth       bool   `json:"use_auth" default:"false"`
	Username      string `json:"username" default:""`
	Password      string `json:"password" default:""`
}

func New() (Config, error) {
	c := Config{}
	path := os.Getenv("CFG_PATH")

	if path == "" {
		path = "./conf/config.json"
	}

	if err := configor.New(&configor.Config{ErrorOnUnmatchedKeys: true}).Load(&c, path); err != nil {
		return c, err
	}

	return c, nil
}
