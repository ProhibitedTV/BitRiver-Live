package pgtype

import "time"

type Text struct {
	String string
	Valid  bool
}

type Int4 struct {
	Int32 int32
	Valid bool
}

type Timestamptz struct {
	Time  time.Time
	Valid bool
}
