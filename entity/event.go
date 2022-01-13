package entity

import (
	"time"
)

type Event struct {
	Message string
	Time    time.Time
	*Meta
}

func NewEvent(line *Line, meta *Meta) *Event {
	return &Event{
		Message: line.Str,
		Time:    line.Time,
		Meta: &Meta{
			PodName:       meta.PodName,
			Namespace:     meta.Namespace,
			ContainerName: meta.ContainerName,
			PodID:         meta.PodID,
		},
	}
}
