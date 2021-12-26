package service

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nxadm/tail"
	"github.com/soulgarden/logfowd/storage"

	"github.com/rs/zerolog"
	"github.com/soulgarden/logfowd/conf"
	"github.com/soulgarden/logfowd/dictionary"
)

const primaryGoroutinesNum = 5

type Watcher struct {
	cfg          *conf.Config
	addCh        chan string
	delCh        chan string
	changes      chan bool
	logs         chan *tail.Line
	hasLog       chan bool
	stateManager *State
	esCli        *Cli
	state        *storage.State
	logger       *zerolog.Logger
}

func NewWatcher(cfg *conf.Config, stateManager *State, esCli *Cli, logger *zerolog.Logger) *Watcher {
	return &Watcher{
		cfg:          cfg,
		addCh:        make(chan string, 1),
		delCh:        make(chan string, 1),
		changes:      make(chan bool, dictionary.FlushChangesChannelSize),
		logs:         make(chan *tail.Line, dictionary.LogsChannelSize),
		hasLog:       make(chan bool, dictionary.LogsChannelSize),
		stateManager: stateManager,
		esCli:        esCli,
		state:        storage.NewState(),
		logger:       logger,
	}
}

func (s *Watcher) Start(ctx context.Context) {
	var primaryWg sync.WaitGroup

	c, cancel := context.WithCancel(ctx)

	if err := s.syncState(c, &primaryWg); err != nil {
		s.logger.Err(err).Msg("sync state")

		cancel()

		return
	}

	primaryWg.Add(primaryGoroutinesNum)

	go func() {
		err := s.statePersister(c, &primaryWg)
		if err != nil {
			s.logger.Err(err).Msg("state persister")

			cancel()
		}
	}()

	go func() {
		err := s.listenAddFile(c, &primaryWg)
		if err != nil {
			s.logger.Err(err).Msg("listen add file")

			cancel()
		}
	}()

	go func() {
		err := s.esSender(c, &primaryWg)
		if err != nil {
			s.logger.Err(err).Msg("es sender")

			cancel()
		}
	}()

	go s.listenDelFile(c, &primaryWg)

	go func() {
		err := s.watchForLogFiles(c, &primaryWg)
		if err != nil {
			s.logger.Err(err).Msg("watch for log files")

			cancel()
		}
	}()

	primaryWg.Wait()

	s.logger.Err(s.stateManager.SaveState(s.state.FlushChanges())).Msg("save state before shutdown")
}

func (s *Watcher) watchForLogFiles(ctx context.Context, wg *sync.WaitGroup) error {
	s.logger.Info().Msg("start log files watcher")

	defer s.logger.Info().Msg("stop log files watcher")
	defer wg.Done()

	for {
		select {
		case <-time.After(dictionary.CheckLogsFilesInterval):
			for _, path := range s.cfg.LogsPath {
				err := s.list(path)
				if err != nil {
					s.logger.Err(err).Msg("list logs dir")

					return err
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *Watcher) list(logPath string) error {
	err := filepath.WalkDir(logPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			s.logger.Err(err).Str("path", path).Msg("list logs dir")

			return err
		}

		if !d.IsDir() {
			if d.Name()[len(d.Name())-4:] == ".log" {
				if !s.state.IsFileExists(path) {
					s.addCh <- path
				}
			}
		}

		return err
	})
	if err != nil {
		s.logger.Err(err).Msg("list logs dir")
	}

	return err
}

func (s *Watcher) listenAddFile(ctx context.Context, wg *sync.WaitGroup) error {
	s.logger.Info().Msg("start add file listener")

	defer s.logger.Info().Msg("stop add file listener")
	defer wg.Done()

	errChan := make(chan error)

	go func() {
		for {
			select {
			case path := <-s.addCh:
				s.state.AddFile(path)

				s.makeChange()

				wg.Add(1)

				go func(c context.Context, wg *sync.WaitGroup, path string) {
					err := s.tailFile(c, wg, path)
					if err != nil {
						errChan <- err
					}
				}(ctx, wg, path)

			case <-ctx.Done():
				close(s.addCh)

				errChan <- nil

				return
			}
		}
	}()

	err := <-errChan

	return err
}

func (s *Watcher) listenDelFile(ctx context.Context, wg *sync.WaitGroup) {
	s.logger.Info().Msg("start del file listener")

	defer s.logger.Info().Msg("stop del file listener")
	defer wg.Done()

	for {
		select {
		case path := <-s.delCh:
			s.logger.Warn().Str("path", path).Msg("del path from listen")

			s.state.DeleteFile(path)

			s.makeChange()

		case <-ctx.Done():
			for len(s.delCh) > 0 {
				path := <-s.delCh

				s.logger.Warn().Str("path", path).Msg("del path from listen")

				s.state.DeleteFile(path)

				s.makeChange()
			}

			close(s.delCh)

			return
		}
	}
}

func (s *Watcher) tailFile(ctx context.Context, wg *sync.WaitGroup, path string) error {
	s.logger.Debug().Str("path", path).Msg("start listen for file")

	defer s.logger.Debug().Str("path", path).Msg("finish listen for file")
	defer wg.Done()

	fileState := s.state.GetFileState(path)

	t, err := tail.TailFile(
		path,
		tail.Config{
			Follow:    true,
			ReOpen:    false,
			MustExist: true,
			Poll:      s.cfg.Poll,
			Location:  &fileState,
			Logger:    log.New(s.logger, "", 0),
		},
	)
	if err != nil {
		s.logger.Err(err).Str("path", path).Msg("tail file")

		return err
	}

	for {
		select {
		case line, ok := <-t.Lines:
			if !ok {
				s.delCh <- path

				return nil
			}

			s.state.UpdateFileState(path, line.SeekInfo)

			s.makeChange()

			s.addLogToBuffer(line)
		case <-ctx.Done():
			err = t.Stop()
			if err != nil {
				s.logger.Err(err).Str("path", path).Msg("stop tail file")
			}

			for len(t.Lines) > 0 {
				line := <-t.Lines

				s.state.UpdateFileState(path, line.SeekInfo)
				s.logger.Info().Msg(line.Text)

				s.makeChange()
			}

			return nil
		}
	}
}

func (s *Watcher) statePersister(ctx context.Context, wg *sync.WaitGroup) error {
	s.logger.Info().Msg("start state persister")

	defer s.logger.Info().Msg("stop state persister")
	defer wg.Done()

	for {
		select {
		case <-time.After(dictionary.FlushStateInterval):
			if s.state.GetChangesNumber() > 0 {
				err := s.stateManager.SaveState(s.state.FlushChanges())
				if err != nil {
					s.logger.Err(err).Msg("save state by timer")

					return err
				}
			}
		case <-s.changes:
			if s.state.GetChangesNumber() > dictionary.FlushChangesNumber {
				err := s.stateManager.SaveState(s.state.FlushChanges())
				if err != nil {
					s.logger.Err(err).Msg("save state by changes number")

					return err
				}
			}

		case <-ctx.Done():
			return nil
		}
	}
}

func (s *Watcher) esSender(ctx context.Context, wg *sync.WaitGroup) error {
	s.logger.Info().Msg("start es sender")

	defer s.logger.Info().Msg("stop es sender")
	defer wg.Done()

	for {
		select {
		case <-time.After(time.Duration(s.cfg.ES.FlushInterval) * time.Millisecond):
			err := s.sendToESByTimer()
			if err != nil {
				return err
			}
		case <-s.hasLog:
			err := s.sendToESByLimit()
			if err != nil {
				return err
			}
		case <-ctx.Done():
			s.logger.Warn().Msg("es sender ctx done")

			logs := []*tail.Line{}

			for len(s.logs) > 0 {
				logs = append(logs, <-s.logs)
			}

			if len(logs) == 0 {
				return nil
			}

			err := s.esCli.SendEvents(logs)

			s.logger.Err(err).Int("num", len(logs)).Msg("send remaining logs to es before shutting down")

			return err
		}
	}
}

func (s *Watcher) sendToESByLimit() error {
	if len(s.logs) < dictionary.FlushLogsNumber {
		return nil
	}

	logs := []*tail.Line{}

	for i := 0; i < dictionary.FlushLogsNumber; i++ {
		logs = append(logs, <-s.logs)
	}

	err := s.esCli.SendEvents(logs)

	s.logger.Err(err).
		Int("num", len(logs)).
		Int("num remaining", len(s.logs)).
		Msg("flushed to es by logs number limit")

	return err
}

func (s *Watcher) sendToESByTimer() error {
	num := len(s.logs)

	if num == 0 {
		return nil
	}

	logs := []*tail.Line{}

	for i := 0; i < num; i++ {
		logs = append(logs, <-s.logs)
	}

	err := s.esCli.SendEvents(logs)

	s.logger.Err(err).
		Int("num", len(logs)).
		Int("num remaining", len(s.logs)).
		Msg("flushed to es by timer")

	return err
}

func (s *Watcher) syncState(ctx context.Context, wg *sync.WaitGroup) error {
	if _, err := os.Stat(s.cfg.StatePath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		s.logger.Err(err).Msg("stat state file")

		return err
	}

	s.logger.Debug().Str("path", s.cfg.StatePath).Msg("load state")

	state, err := s.stateManager.LoadState()
	if err != nil {
		s.logger.Err(err).Msg("load state")

		return err
	}

	s.state.Load(state)

	for path := range state {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				s.logger.Debug().Str("path", path).Msg("file not found, delete from storage")
			} else {
				s.logger.Err(err).Str("path", path).Msg("stat file")
			}

			s.state.DeleteFile(path)
		} else {
			wg.Add(1)

			go func(ctx context.Context, wg *sync.WaitGroup, path string) {
				_ = s.tailFile(ctx, wg, path)
			}(ctx, wg, path)
		}
	}

	return nil
}

func (s *Watcher) makeChange() {
	if len(s.changes) == dictionary.FlushChangesChannelSize {
		return
	}

	s.changes <- true
}

func (s *Watcher) addLogToBuffer(log *tail.Line) {
	if len(s.logs) == dictionary.LogsChannelSize {
		s.logger.Err(dictionary.ErrChannelOverflowed).Msg(dictionary.ErrChannelOverflowed.Error())

		return
	}

	s.logs <- log

	if len(s.hasLog) != dictionary.LogsChannelSize {
		s.hasLog <- true
	}
}
