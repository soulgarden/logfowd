package service

import (
	"encoding/json"
	"io/ioutil"

	"github.com/soulgarden/logfowd/service/file"

	"github.com/rs/zerolog"
	"github.com/soulgarden/logfowd/conf"
	"github.com/soulgarden/logfowd/dictionary"
)

type State struct {
	cfg    *conf.Config
	logger *zerolog.Logger
}

func NewState(cfg *conf.Config, logger *zerolog.Logger) *State {
	return &State{cfg: cfg, logger: logger}
}

func (s *State) SaveState(state map[string]*file.File) error {
	marshalled, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(s.cfg.StatePath, marshalled, dictionary.DumpFilePermissions)
}

func (s *State) LoadFiles() (map[string]*file.File, error) {
	var state map[string]*file.File

	data, err := ioutil.ReadFile(s.cfg.StatePath)
	if err != nil {
		s.logger.Err(err).Msg("read state file")

		return nil, err
	}

	err = json.Unmarshal(data, &state)
	if err != nil {
		s.logger.Err(err).Bytes("data", data).Msg("unmarshall state")

		return nil, err
	}

	return state, nil
}
