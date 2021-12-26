package storage

import (
	"sync"

	"github.com/nxadm/tail"
)

type State struct {
	mx            sync.RWMutex
	files         map[string]tail.SeekInfo
	changesNumber uint64
}

func NewState() *State {
	return &State{
		files: make(map[string]tail.SeekInfo),
	}
}

func (s *State) AddFile(path string) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.files[path] = tail.SeekInfo{}

	s.changesNumber++
}

func (s *State) GetFileState(path string) tail.SeekInfo {
	s.mx.RLock()
	defer s.mx.RUnlock()

	return s.files[path]
}

func (s *State) DeleteFile(path string) {
	s.mx.Lock()
	defer s.mx.Unlock()

	delete(s.files, path)
	s.changesNumber++
}

func (s *State) UpdateFileState(path string, state tail.SeekInfo) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.files[path] = state
	s.changesNumber++
}

func (s *State) IsFileExists(path string) bool {
	s.mx.RLock()
	defer s.mx.RUnlock()

	if _, ok := s.files[path]; ok {
		return true
	}

	return false
}

func (s *State) GetChangesNumber() uint64 {
	s.mx.RLock()
	defer s.mx.RUnlock()

	return s.changesNumber
}

func (s *State) FlushChanges() map[string]tail.SeekInfo {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.changesNumber = 0

	files := make(map[string]tail.SeekInfo, len(s.files))

	for k, v := range s.files {
		files[k] = v
	}

	return files
}

func (s *State) Load(state map[string]tail.SeekInfo) {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.changesNumber = 0

	s.files = state
}
