package storage

import (
	"sync"

	"github.com/soulgarden/logfowd/dictionary"

	"github.com/nxadm/tail"
	"github.com/soulgarden/logfowd/entity"
)

type State struct {
	mx     sync.RWMutex
	files  map[string]*entity.State
	change chan struct{}
}

func NewState() *State {
	return &State{
		files:  make(map[string]*entity.State),
		change: make(chan struct{}, dictionary.FlushChangesChannelSize),
	}
}

func (s *State) ListenChange() <-chan struct{} {
	return s.change
}

func (s *State) AddFile(path string, state *entity.State) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.files[path] = state

	s.makeChange()
}

func (s *State) GetFileState(path string) *entity.State {
	s.mx.RLock()
	defer s.mx.RUnlock()

	return s.files[path]
}

func (s *State) DeleteFile(path string) {
	s.mx.Lock()
	defer s.mx.Unlock()

	delete(s.files, path)
	s.makeChange()
}

func (s *State) UpdateFileSeekInfo(path string, seekInfo *tail.SeekInfo) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.files[path].SeekInfo = seekInfo
	s.makeChange()
}

func (s *State) IsFileExists(path string) bool {
	s.mx.RLock()
	defer s.mx.RUnlock()

	if _, ok := s.files[path]; ok {
		return true
	}

	return false
}

func (s *State) FlushChanges(number int) map[string]*entity.State {
	s.mx.Lock()
	defer s.mx.Unlock()

	for i := 0; i < number; i++ {
		<-s.change
	}

	files := make(map[string]*entity.State, len(s.files))

	for k, v := range s.files {
		files[k] = v
	}

	return files
}

func (s *State) Load(state map[string]*entity.State) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.files = state
}

func (s *State) makeChange() {
	if len(s.change) == cap(s.change) {
		return
	}

	s.change <- struct{}{}
}
