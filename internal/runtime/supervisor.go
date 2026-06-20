package runtime

import (
	"context"
	"log/slog"
	"sync"

	"github.com/radcolor/trishna-go/internal/modules"
)

func RunServices(ctx context.Context, logger *slog.Logger, services ...modules.BackgroundService) error {
	if logger == nil {
		logger = slog.Default()
	}

	var wg sync.WaitGroup
	for _, service := range services {
		if service == nil {
			continue
		}
		wg.Add(1)
		go func(svc modules.BackgroundService) {
			defer wg.Done()
			if err := svc.Run(ctx); err != nil && ctx.Err() == nil {
				logger.Error("service stopped",
					slog.String("service", svc.Name()),
					slog.String("error", err.Error()),
				)
			}
		}(service)
	}

	<-ctx.Done()
	wg.Wait()
	return nil
}
