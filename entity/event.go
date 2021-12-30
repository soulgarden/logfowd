package entity

import (
	"time"

	"github.com/nxadm/tail"
)

type Event struct {
	Message string
	Time    time.Time
	*Meta
}

func NewEvent(line *tail.Line, meta *Meta) *Event {
	return &Event{
		Message: line.Text,
		Time:    line.Time,
		Meta: &Meta{
			PodName:       meta.PodName,
			Namespace:     meta.Namespace,
			ContainerName: meta.ContainerName,
			ContainerID:   meta.ContainerID,
		},
	}
}
