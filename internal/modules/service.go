package modules

import "context"

type BackgroundService interface {
	Name() string
	Run(ctx context.Context) error
}
