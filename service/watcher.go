package service

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/soulgarden/logfowd/entity"

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
	change       chan bool
	event        chan *entity.Event
	esEvents     chan []*entity.Event
	hasEvent     chan bool
	stateManager *State
	esCli        *Cli
	k8sRegexp    *regexp.Regexp
	state        *storage.State
	logger       *zerolog.Logger
}

func NewWatcher(cfg *conf.Config, stateManager *State, esCli *Cli, logger *zerolog.Logger) *Watcher {
	return &Watcher{
		cfg:          cfg,
		addCh:        make(chan string, 1),
		delCh:        make(chan string, 1),
		change:       make(chan bool, dictionary.FlushChangesChannelSize),
		event:        make(chan *entity.Event, dictionary.LogsChannelSize),
		esEvents:     make(chan []*entity.Event, cfg.ES.Workers*dictionary.SendBatchesNum),
		hasEvent:     make(chan bool, dictionary.LogsChannelSize),
		stateManager: stateManager,
		esCli:        esCli,
		k8sRegexp:    regexp.MustCompile(dictionary.K8sRegexp),
		state:        storage.NewState(),
		logger:       logger,
	}
}

func (s *Watcher) Start(ctx context.Context) {
	var wg sync.WaitGroup

	c, cancel := context.WithCancel(ctx)

	if err := s.syncState(c, &wg); err != nil {
		s.logger.Err(err).Msg("sync state")

		cancel()

		return
	}

	wg.Add(primaryGoroutinesNum + s.cfg.ES.Workers)

	go func() {
		err := s.statePersister(c, &wg)
		if err != nil {
			s.logger.Err(err).Msg("state persister")

			cancel()
		}
	}()

	go func() {
		err := s.listenAddFile(c, &wg)
		if err != nil {
			s.logger.Err(err).Msg("listen add file")

			cancel()
		}
	}()

	go s.esSendDispatcher(c, &wg)

	for i := 0; i < s.cfg.ES.Workers; i++ {
		go func(c context.Context, wg *sync.WaitGroup) {
			err := s.esSender(c, wg)
			if err != nil {
				s.logger.Err(err).Msg("es sender")

				cancel()
			}
		}(c, &wg)
	}

	go s.listenDelFile(c, &wg)

	go func() {
		err := s.watchForLogFiles(c, &wg)
		if err != nil {
			s.logger.Err(err).Msg("watch for log files")

			cancel()
		}
	}()

	wg.Wait()

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
					s.logger.Err(err).Msg("list event dir")

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
			s.logger.Err(err).Str("path", path).Msg("list event dir")

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
		s.logger.Err(err).Msg("list event dir")
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
				s.state.AddFile(path, &entity.State{
					SeekInfo: nil,
					Meta:     s.parseK8sMeta(path),
				})

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
			Location:  fileState.SeekInfo,
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

			s.state.UpdateFileSeekInfo(path, &line.SeekInfo)

			s.makeChange()

			s.addLogToBuffer(entity.NewEvent(line, fileState.Meta))
		case <-ctx.Done():
			err = t.Stop()
			if err != nil {
				s.logger.Err(err).Str("path", path).Msg("stop tail file")
			}

			for len(t.Lines) > 0 {
				line := <-t.Lines

				s.state.UpdateFileSeekInfo(path, &line.SeekInfo)

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
		case <-s.change:
			if s.state.GetChangesNumber() > dictionary.FlushChangesNumber {
				err := s.stateManager.SaveState(s.state.FlushChanges())
				if err != nil {
					s.logger.Err(err).Msg("save state by change number")

					return err
				}
			}

		case <-ctx.Done():
			return nil
		}
	}
}

func (s *Watcher) esSendDispatcher(ctx context.Context, wg *sync.WaitGroup) {
	s.logger.Info().Msg("start es send dispatcher")

	defer s.logger.Info().Msg("stop es send dispatcher")
	defer wg.Done()

	for {
		select {
		case <-time.After(time.Duration(s.cfg.ES.FlushInterval) * time.Millisecond):
			s.sendToESByTimer()
		case <-s.hasEvent:
			s.sendToESByLimit()
		case <-ctx.Done():
			s.sendToESRemainingEvents()

			return
		}
	}
}

func (s *Watcher) esSender(ctx context.Context, wg *sync.WaitGroup) error {
	s.logger.Info().Msg("start es sender")

	defer s.logger.Info().Msg("stop es sender")
	defer wg.Done()

	for {
		select {
		case events := <-s.esEvents:
			err := s.esCli.SendEvents(events)

			s.logger.Err(err).Int("num", len(events)).Msg("send events to es")

			if err != nil {
				return err
			}
		case <-ctx.Done():
			for len(s.esEvents) > 0 {
				events := <-s.esEvents

				err := s.esCli.SendEvents(events)

				s.logger.Err(err).Int("num", len(events)).Msg("send remaining event to es before shutting down")
			}

			return nil
		}
	}
}

func (s *Watcher) sendToESByLimit() {
	if len(s.event) < dictionary.FlushLogsNumber {
		return
	}

	events := []*entity.Event{}

	for i := 0; i < dictionary.FlushLogsNumber; i++ {
		events = append(events, <-s.event)
	}

	s.esEvents <- events

	s.logger.Debug().
		Int("num", len(events)).
		Int("num remaining", len(s.event)).
		Msg("flushed event to es senders by event number limit")
}

func (s *Watcher) sendToESByTimer() {
	num := len(s.event)

	if num == 0 {
		return
	}

	events := []*entity.Event{}

	for i := 0; i < num; i++ {
		events = append(events, <-s.event)
	}

	s.esEvents <- events

	s.logger.Warn().
		Int("num", len(events)).
		Int("num remaining", len(s.event)).
		Msg("flushed event to es senders by timer")
}

func (s *Watcher) sendToESRemainingEvents() {
	events := []*entity.Event{}

	if len(s.event) == 0 {
		return
	}

	for len(s.event) > 0 {
		events = append(events, <-s.event)
	}

	s.esEvents <- events

	s.logger.Warn().Int("num", len(events)).Msg("send remaining event to es senders before shutting down")
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
	if len(s.change) == dictionary.FlushChangesChannelSize {
		return
	}

	s.change <- true
}

func (s *Watcher) addLogToBuffer(event *entity.Event) {
	if len(s.event) == dictionary.LogsChannelSize {
		s.logger.Err(dictionary.ErrChannelOverflowed).Msg(dictionary.ErrChannelOverflowed.Error())

		return
	}

	s.event <- event

	if len(s.hasEvent) != dictionary.LogsChannelSize {
		s.hasEvent <- true
	}
}

func (s *Watcher) parseK8sMeta(path string) *entity.Meta {
	matches := s.k8sRegexp.FindAllStringSubmatch(path, -1)

	if matches == nil {
		return &entity.Meta{}
	}

	return &entity.Meta{
		PodName:       matches[0][1],
		Namespace:     matches[0][2],
		ContainerName: matches[0][3],
		ContainerID:   matches[0][4],
	}
}
