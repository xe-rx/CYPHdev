package engine

type EventKind string

const (
	Put      EventKind = "put"
	Delete   EventKind = "delete"
	Snapshot EventKind = "snapshot"
)

type Event struct {
	Kind        EventKind
	Key         string
	Value       []byte
	ModRevision int64
}
