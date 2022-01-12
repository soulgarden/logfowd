package storage

import (
	"sync"

	"github.com/soulgarden/logfowd/service/file"

	"github.com/soulgarden/logfowd/dictionary"
)

type State struct {
	mx            sync.RWMutex
	files         map[string]*file.File
	change        chan struct{}
	changesNumber uint64
}

func NewState() *State {
	return &State{
		files:  make(map[string]*file.File),
		change: make(chan struct{}, dictionary.FlushChangesChannelSize),
	}
}

func (s *State) ListenChange() <-chan struct{} {
	return s.change
}

func (s *State) SetFile(path string, state *file.File) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.files[path] = state

	s.makeChange()
}

func (s *State) RenameFile(oldPath, newPath string) {
	s.mx.Lock()
	defer s.mx.Unlock()

	f := s.files[oldPath]
	delete(s.files, oldPath)

	f.Path = newPath

	s.files[newPath] = f

	s.makeChange()
}

func (s *State) GetFile(path string) *file.File {
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

func (s *State) IsFileExists(path string) bool {
	s.mx.RLock()
	defer s.mx.RUnlock()

	if _, ok := s.files[path]; ok {
		return true
	}

	return false
}

func (s *State) FlushChanges(number int) map[string]*file.File {
	s.mx.Lock()
	defer s.mx.Unlock()

	for i := 0; i < number; i++ {
		<-s.change
		s.changesNumber--
	}

	files := make(map[string]*file.File, len(s.files))

	for k, v := range s.files {
		files[k] = v
	}

	return files
}

func (s *State) Load(state map[string]*file.File) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.files = state
}

func (s *State) makeChange() {
	if len(s.change) == cap(s.change) {
		return
	}

	s.changesNumber++
	s.change <- struct{}{}
}

func (s *State) GetChangesNumber() uint64 {
	s.mx.RLock()
	defer s.mx.RUnlock()

	return s.changesNumber
}
