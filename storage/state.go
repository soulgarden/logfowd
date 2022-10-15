package storage

import (
	"sync"

	"github.com/soulgarden/logfowd/service/file"
)

type State struct {
	mx    sync.RWMutex
	files map[string]*file.File
}

func NewState() *State {
	return &State{
		files: make(map[string]*file.File),
	}
}

func (s *State) SetFile(path string, files *file.File) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.files[path] = files
}

func (s *State) RenameFile(oldPath, newPath string) {
	s.mx.Lock()
	defer s.mx.Unlock()

	f := s.files[oldPath]
	delete(s.files, oldPath)

	f.EntityFile.Path = newPath

	s.files[newPath] = f
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
}

func (s *State) IsFileExists(path string) bool {
	s.mx.RLock()
	defer s.mx.RUnlock()

	if _, ok := s.files[path]; ok {
		return true
	}

	return false
}
