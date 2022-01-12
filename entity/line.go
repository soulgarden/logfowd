package entity

import "time"

type Line struct {
	Num  int64
	Pos  int64
	Str  string
	Time time.Time
}
