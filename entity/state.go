package entity

import "github.com/nxadm/tail"

type State struct {
	*tail.SeekInfo
	*Meta
}
