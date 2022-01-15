package storage

import (
	"sync"

	"github.com/soulgarden/logfowd/entity"
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

func (s *State) SetFile(path string, files *file.File) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.files[path] = files

	s.makeChange()
}

func (s *State) RenameFile(oldPath, newPath string) {
	s.mx.Lock()
	defer s.mx.Unlock()

	f := s.files[oldPath]
	delete(s.files, oldPath)

	f.EntityFile.Path = newPath

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

func (s *State) FlushState(number int) *entity.State {
	s.mx.Lock()
	defer s.mx.Unlock()

	for i := 0; i < number; i++ {
		<-s.change
		s.changesNumber--
	}

	state := entity.NewState(len(s.files))

	for k, v := range s.files {
		state.Files[k] = v.EntityFile
	}

	return state
}

func (s *State) Load(state *entity.State) map[string]*file.File {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.files = make(map[string]*file.File, len(state.Files))

	for k, v := range state.Files {
		s.files[k] = &file.File{
			EntityFile: v,
		}
	}

	return s.files
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
