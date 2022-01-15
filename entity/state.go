package entity

type State struct {
	Files map[string]*File
}

func NewState(size int) *State {
	return &State{Files: make(map[string]*File, size)}
}
