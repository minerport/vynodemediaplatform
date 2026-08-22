package events

import "context"

type Event struct {
	Type    string `json:"type"`
	Version int    `json:"version"`
	Data    any    `json:"data"`
}
type Publisher interface {
	Publish(context.Context, Event) error
}
type Subscriber interface {
	Subscribe(context.Context, ...string) (<-chan Event, error)
}
