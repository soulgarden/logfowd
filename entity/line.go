package entity

import "time"

type Line struct {
	Pos  int64
	Str  string
	Time time.Time
}
