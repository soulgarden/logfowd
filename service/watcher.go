package service

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/soulgarden/logfowd/service/file"

	"github.com/fsnotify/fsnotify"
	"github.com/soulgarden/logfowd/dictionary"

	"github.com/soulgarden/logfowd/entity"

	"github.com/soulgarden/logfowd/storage"

	"github.com/rs/zerolog"
	"github.com/soulgarden/logfowd/conf"
	"golang.org/x/sync/errgroup"
)

type Watcher struct {
	cfg          *conf.Config
	event        chan *entity.Event
	esEvents     chan []*entity.Event
	hasEvent     chan struct{}
	stateManager *State
	esCli        *Cli
	k8sRegexp    *regexp.Regexp
	state        *storage.State
	logger       *zerolog.Logger
}

func NewWatcher(cfg *conf.Config, stateManager *State, esCli *Cli, logger *zerolog.Logger) *Watcher {
	return &Watcher{
		cfg:          cfg,
		event:        make(chan *entity.Event, cfg.ES.Workers*dictionary.SendBatchesNum*dictionary.FlushLogsNumber),
		esEvents:     make(chan []*entity.Event, cfg.ES.Workers*dictionary.SendBatchesNum),
		hasEvent:     make(chan struct{}, cfg.ES.Workers*dictionary.SendBatchesNum*dictionary.FlushLogsNumber),
		stateManager: stateManager,
		esCli:        esCli,
		k8sRegexp:    regexp.MustCompile(dictionary.K8sPodsRegexp),
		state:        storage.NewState(),
		logger:       logger,
	}
}

func (s *Watcher) Start(ctx context.Context) {
	g, ctx := errgroup.WithContext(ctx)

	if err := s.syncState(ctx, g); err != nil {
		s.logger.Err(err).Msg("sync state")

		return
	}

	if err := s.stateManager.Open(); err != nil {
		s.logger.Err(err).Msg("open storage file")

		return
	}

	defer s.stateManager.Close()

	g.Go(func() error {
		return s.statePersister(ctx)
	})

	g.Go(func() error {
		return s.esSendDispatcher(ctx)
	})

	for i := 0; i < s.cfg.ES.Workers; i++ {
		i := i

		g.Go(func() error {
			return s.esSender(ctx, i)
		})
	}

	if err := s.syncFiles(ctx, g); err != nil {
		s.logger.Err(err).Msg("sync files")

		return
	}

	g.Go(func() error {
		return s.watch(ctx, g)
	})

	err := g.Wait()

	s.logger.Err(err).Msg("wait goroutines")

	s.logger.Err(s.stateManager.SaveState(s.state.FlushState(0))).Msg("save state before shutdown")
}

// nolint: funlen, gocognit, cyclop
func (s *Watcher) watch(ctx context.Context, g *errgroup.Group) error {
	s.logger.Debug().Msg("start log files watcher")

	defer s.logger.Debug().Msg("stop log files watcher")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		s.logger.Err(err).Msg("new watcher")

		return err
	}

	defer watcher.Close()

	if err := s.addWatchers(watcher); err != nil {
		s.logger.Err(err).Msg("add watchers")

		return err
	}

	var oldRenamedPath string

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				s.logger.Err(dictionary.ErrChannelClosed).Msg("watcher events channel closed")

				return dictionary.ErrChannelClosed
			}

			switch {
			case event.Op&fsnotify.Create == fsnotify.Create:
				if oldRenamedPath != "" {
					s.logger.Warn().
						Str("before path", oldRenamedPath).
						Str("after path", event.Name).
						Msg("file renamed")

					s.fileRenamed(oldRenamedPath, event.Name)
					oldRenamedPath = ""

					continue
				}

				if err := s.created(ctx, g, watcher, event.Name); err != nil {
					s.logger.Err(err).Str("path", event.Name).Msg("created")

					return err
				}
			case event.Op&fsnotify.Rename == fsnotify.Rename:
				if !s.isLogFile(event.Name) {
					continue
				}

				s.logger.Warn().Str("old path", event.Name).Msg("file renamed, save old path")

				oldRenamedPath = event.Name
			case event.Op&fsnotify.Remove == fsnotify.Remove:
				if err := s.deleted(event.Name); err != nil {
					s.logger.Err(err).Str("path", event.Name).Msg("deleted")

					return err
				}
			case event.Op&fsnotify.Write == fsnotify.Write:
				if err := s.written(event.Name); err != nil {
					s.logger.Err(err).Str("path", event.Name).Msg("written")

					return err
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				s.logger.Err(err).Msg("watcher error channel closed")

				return nil
			}

			s.logger.Err(err).Msg("watcher received error")
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *Watcher) syncFiles(ctx context.Context, g *errgroup.Group) error {
	for _, path := range s.cfg.LogsPath {
		err := s.list(ctx, g, path)
		if err != nil {
			s.logger.Err(err).Msg("list event dir")

			return err
		}
	}

	return nil
}

func (s *Watcher) addWatchers(watcher *fsnotify.Watcher) error {
	for _, path := range s.cfg.LogsPath {
		err := watcher.Add(path)
		s.logger.Err(err).Str("path", path).Msg("add to watcher")

		if err != nil {
			return err
		}

		err = s.addWatchersRecursive(watcher, path)

		s.logger.Err(err).Msg("walk dir")

		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Watcher) addWatchersRecursive(watcher *fsnotify.Watcher, rootPath string) error {
	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			s.logger.Err(err).Str("path", path).Msg("list dir")

			return err
		}

		if path == rootPath {
			return nil
		}

		if d.IsDir() {
			s.logger.Err(err).Str("path", path).Str("name", d.Name()).Msg("list dir")

			err = watcher.Add(path)
			s.logger.Err(err).Str("path", path).Msg("add to watcher")
			if err != nil {
				return err
			}

			if err := s.addWatchersRecursive(watcher, path); err != nil {
				s.logger.Err(err).Str("path", path).Msg("add watchers recursive")

				return err
			}
		}

		return err
	})

	s.logger.Err(err).Str("path", rootPath).Msg("walk dir")

	return err
}

func (s *Watcher) created(
	ctx context.Context,
	g *errgroup.Group,
	watcher *fsnotify.Watcher,
	path string,
) error {
	info, err := s.statFile(path)
	if err != nil {
		return err
	}

	info.Size()

	if info.IsDir() {
		err = watcher.Add(path)
		s.logger.Err(err).Str("path", path).Msg("add to watcher")

		if err != nil {
			return err
		}

		err := s.addWatchersRecursive(watcher, path)
		if err != nil {
			s.logger.Err(err).Str("path", path).Msg("new watcher")
		}

		return err
	}

	if !s.isLogFile(path) {
		return nil
	}

	if !s.state.IsFileExists(path) {
		f, err := s.addFile(path)
		if err != nil {
			return err
		}

		g.Go(func() error {
			return s.listenLine(ctx, f)
		})

		err = f.Read()
		if err != nil {
			s.logger.Err(err).Str("path", path).Msg("read file")
		}

		return err
	}

	return nil
}

func (s *Watcher) written(path string) error {
	if !s.isLogFile(path) {
		return nil
	}

	f := s.state.GetFile(path)
	if f == nil {
		s.logger.Warn().Str("path", path).Msg("file not exist in storage")

		return nil
	}

	isTrunc, err := f.IsTruncated()
	if err != nil {
		s.logger.Err(err).
			Str("path", path).
			Msg("is truncated")

		return err
	}

	if isTrunc {
		err = f.Reset()
		if err != nil {
			s.logger.Err(err).
				Str("path", path).
				Msg("reopen truncated file")

			return err
		}

		s.logger.Warn().
			Str("path", path).
			Int64("offset", f.EntityFile.Offset).
			Int64("size", f.EntityFile.Size).
			Msg("file was truncated")

		s.state.SetFile(path, f)
	}

	err = f.Read()
	if err != nil {
		s.logger.Err(err).
			Str("path", path).
			Msg("read line")
	}

	return err
}

func (s *Watcher) fileRenamed(oldPath, newPath string) {
	s.state.RenameFile(oldPath, newPath)
}

func (s *Watcher) deleted(path string) error {
	if !s.isLogFile(path) {
		return nil
	}

	if f := s.state.GetFile(path); f == nil {
		s.logger.Warn().Str("path", path).Msg("file not exist in storage, was it a folder?")

		return nil
	}

	err := s.deleteFile(path)
	if err != nil {
		s.logger.Err(err).Str("path", path).Msg("read line")
	}

	return err
}

func (s *Watcher) addFile(path string) (*file.File, error) {
	f, err := file.NewFile(path)

	s.logger.Err(err).Str("path", path).Msg("add file")

	if err != nil {
		return nil, err
	}

	f.EntityFile.Meta = s.parseK8sMeta(path)

	s.state.SetFile(path, f)

	s.logger.Err(err).Str("path", path).Msg("add file")

	return f, nil
}

func (s *Watcher) deleteFile(path string) error {
	s.logger.Info().Str("path", path).Msg("delete file")

	f := s.state.GetFile(path)
	if f == nil {
		s.logger.Info().Str("path", path).Msg("file not found in storage")

		return nil
	}

	err := f.Close()
	s.logger.Err(err).Str("path", path).Msg("close file")

	s.state.DeleteFile(path)

	return err
}

func (s *Watcher) list(ctx context.Context, g *errgroup.Group, logPath string) error {
	err := filepath.WalkDir(logPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			s.logger.Err(err).Str("path", path).Msg("list event dir")

			return err
		}

		if d.IsDir() && !s.isLogFile(d.Name()) {
			return nil
		}

		if !s.state.IsFileExists(path) {
			f, err := s.addFile(path)
			if err != nil {
				return err
			}

			err = f.SeekEnd()
			if err != nil {
				return err
			}

			s.state.SetFile(path, f)

			g.Go(func() error {
				return s.listenLine(ctx, f)
			})

			g.Go(func() error {
				err = f.Read()
				if err != nil {
					s.logger.Err(err).Str("path", path).Msg("read file")
				}

				return err
			})
		}

		return err
	})
	if err != nil {
		s.logger.Err(err).Msg("list event dir")
	}

	return err
}

func (s *Watcher) statePersister(ctx context.Context) error {
	s.logger.Debug().Msg("start state persister")

	defer s.logger.Debug().Msg("stop state persister")

	for {
		select {
		case <-time.After(dictionary.FlushStateInterval):
			if s.state.GetChangesNumber() > 0 {
				err := s.stateManager.SaveState(s.state.FlushState(len(s.state.ListenChange())))
				if err != nil {
					s.logger.Err(err).Msg("save state by timer")

					return err
				}
			}
		case <-s.state.ListenChange():
			if s.state.GetChangesNumber() >= dictionary.FlushChangesNumber {
				err := s.stateManager.SaveState(s.state.FlushState(len(s.state.ListenChange())))
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

func (s *Watcher) esSendDispatcher(ctx context.Context) error {
	s.logger.Debug().Msg("start es send dispatcher")

	defer s.logger.Debug().Msg("stop es send dispatcher")

	for {
		select {
		case <-time.After(time.Duration(s.cfg.ES.FlushInterval) * time.Millisecond):
			s.sendToESByTimer()
		case <-s.hasEvent:
			s.sendToESByLimit()
		case <-ctx.Done():
			s.sendToESRemainingEvents()

			return nil
		}
	}
}

func (s *Watcher) esSender(ctx context.Context, i int) error {
	s.logger.Debug().Int("worker", i).Msgf("start es sender %d", i)

	defer s.logger.Debug().Int("worker", i).Msgf("stop es sender %d", i)

	for {
		select {
		case events := <-s.esEvents:
			err := s.esCli.SendEvents(events)

			s.logger.Err(err).
				Int("worker", i).
				Int("num", len(events)).
				Msg("send events to es")

			if err != nil {
				return err
			}
		case <-ctx.Done():
			for len(s.esEvents) > 0 {
				events := <-s.esEvents

				err := s.esCli.SendEvents(events)

				s.logger.Err(err).
					Int("worker", i).
					Int("num", len(events)).
					Msg("send remaining event to es before shutting down")
			}

			return nil
		}
	}
}

func (s *Watcher) sendToESByLimit() {
	if len(s.event) < dictionary.FlushLogsNumber {
		return
	}

	events := make([]*entity.Event, dictionary.FlushLogsNumber)

	for i := 0; i < dictionary.FlushLogsNumber; i++ {
		events[i] = <-s.event
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

	events := make([]*entity.Event, num)

	for i := 0; i < num; i++ {
		events[i] = <-s.event
	}

	s.esEvents <- events

	s.logger.Info().
		Int("num", len(events)).
		Int("num remaining", len(s.event)).
		Msg("flushed event to es senders by timer")
}

func (s *Watcher) sendToESRemainingEvents() {
	if len(s.event) == 0 {
		return
	}

	events := []*entity.Event{}

	for len(s.event) > 0 {
		events = append(events, <-s.event)
	}

	s.esEvents <- events

	s.logger.Warn().Int("num", len(events)).Msg("send remaining event to es senders before shutting down")
}

func (s *Watcher) syncState(ctx context.Context, g *errgroup.Group) error {
	if _, err := os.Stat(s.cfg.StatePath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		s.logger.Err(err).Msg("stat files file")

		return err
	}

	s.logger.Debug().Str("path", s.cfg.StatePath).Msg("load files")

	state, err := s.stateManager.LoadState()
	if err != nil {
		s.logger.Err(err).Msg("load state")

		return err
	}

	files := s.state.Load(state)

	for path, f := range files {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				s.logger.Warn().Str("path", path).Msg("file not found, delete from storage")

				s.state.DeleteFile(path)

				continue
			}

			s.logger.Err(err).Str("path", path).Msg("stat file")

			return err
		}

		err = f.RestorePosition()

		s.logger.Err(err).Str("path", path).Msg("restore position")

		if err != nil {
			return err
		}

		f := f

		g.Go(func() error {
			return s.listenLine(ctx, f)
		})

		err = f.Read()
		if err != nil {
			s.logger.Err(err).Msg("read file")

			return err
		}
	}

	return nil
}

func (s *Watcher) listenLine(ctx context.Context, f *file.File) error {
	s.logger.Debug().Str("path", f.EntityFile.Path).Msg("start listen new lines")

	defer s.logger.Debug().Str("path", f.EntityFile.Path).Msg("stop listen new lines")

	fileState := s.state.GetFile(f.EntityFile.Path)

	for {
		select {
		case line, ok := <-f.ListenLine():
			if !ok {
				s.logger.Warn().Str("path", f.EntityFile.Path).Msg("line channel closed")

				return nil
			}

			s.addLogToBuffer(entity.NewEvent(line, fileState.EntityFile.Meta))
			s.state.SetFile(f.EntityFile.Path, f)

		case <-ctx.Done():
			for len(f.ListenLine()) > 0 {
				line := <-f.ListenLine()

				s.addLogToBuffer(entity.NewEvent(line, fileState.EntityFile.Meta))
				s.state.SetFile(f.EntityFile.Path, f)
			}

			return nil
		}
	}
}

func (s *Watcher) addLogToBuffer(event *entity.Event) {
	if len(s.event) == cap(s.event) {
		s.logger.
			Err(dictionary.ErrChannelOverflowed).
			Msg("logs channel overflowed, consider increasing es workers")
	}

	s.event <- event

	if len(s.hasEvent) != cap(s.hasEvent) {
		s.hasEvent <- struct{}{}
	}
}

func (s *Watcher) parseK8sMeta(path string) *entity.Meta {
	matches := s.k8sRegexp.FindAllStringSubmatch(path, -1)

	if matches == nil {
		return &entity.Meta{}
	}

	return &entity.Meta{
		PodName:       matches[0][2],
		Namespace:     matches[0][1],
		ContainerName: matches[0][4],
		PodID:         matches[0][3],
	}
}

func (s *Watcher) isLogFile(path string) bool {
	return len(path) > 4 && path[len(path)-4:] == ".log"
}

func (s *Watcher) statFile(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		s.logger.Err(err).Str("path", path).Msg("stat file/dir")

		return nil, err
	}

	return info, err
}
