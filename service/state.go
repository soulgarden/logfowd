package service

import (
	"io"
	"os"

	"github.com/soulgarden/logfowd/dictionary"
	"github.com/soulgarden/logfowd/entity"

	"github.com/mailru/easyjson"
	"github.com/rs/zerolog"
	"github.com/soulgarden/logfowd/conf"
)

type State struct {
	cfg    *conf.Config
	logger *zerolog.Logger
	file   *os.File
}

func NewState(cfg *conf.Config, logger *zerolog.Logger) *State {
	return &State{cfg: cfg, logger: logger}
}

func (s *State) SaveState(state *entity.State) error {
	marshalled, err := easyjson.Marshal(state)
	if err != nil {
		return err
	}

	err = s.file.Truncate(0)
	if err != nil {
		return err
	}

	_, err = s.file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	_, err = s.file.Write(marshalled)
	if err != nil {
		return err
	}

	return s.file.Sync()
}

func (s *State) LoadState() (*entity.State, error) {
	state := &entity.State{}

	data, err := os.ReadFile(s.cfg.StatePath)
	if err != nil {
		s.logger.Err(err).Msg("read state file")

		return nil, err
	}

	err = easyjson.Unmarshal(data, state)
	if err != nil {
		s.logger.Err(err).Bytes("data", data).Msg("unmarshall state")

		return nil, err
	}

	return state, nil
}

func (s *State) Open() error {
	f, err := os.OpenFile(s.cfg.StatePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, dictionary.DumpFilePermissions)
	s.logger.Err(err).Msg("open storage file")

	s.file = f

	return err
}

func (s *State) Close() error {
	return s.file.Close()
}
