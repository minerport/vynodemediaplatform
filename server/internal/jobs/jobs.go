package jobs

import "context"

type Job interface {
	Name() string
	Run(context.Context) error
}
type Queue interface {
	Submit(context.Context, Job) error
	Shutdown(context.Context) error
}
