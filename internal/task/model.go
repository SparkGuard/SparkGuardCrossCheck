package task

import "time"

type WorkEntry struct {
	WorkID    uint64
	Path      string
	Timestamp time.Time
}

type WorkUrl struct {
	WorkID uint64
	Url    string
}
