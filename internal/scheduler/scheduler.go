package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/lihongjie0209/file-service/internal/cache"
	"github.com/lihongjie0209/file-service/internal/config"
	filedomain "github.com/lihongjie0209/file-service/internal/file"
	"github.com/lihongjie0209/file-service/internal/observability"
	"github.com/robfig/cron/v3"
	"go.uber.org/fx"
)

func New(lc fx.Lifecycle, cfg config.Config, locker *cache.Locker, files *filedomain.Service, metrics *observability.Metrics, logger *slog.Logger) (*cron.Cron, error) {
	location, err := time.LoadLocation(cfg.Cron.Timezone)
	if err != nil {
		return nil, err
	}
	runner := cron.New(cron.WithLocation(location), cron.WithSeconds(), cron.WithChain(cron.Recover(cron.PrintfLogger(slogWriter{logger})), cron.SkipIfStillRunning(cron.PrintfLogger(slogWriter{logger}))))
	if cfg.ObjectStorage.CleanupSpec != "" {
		if _, err := runner.AddFunc(cfg.ObjectStorage.CleanupSpec, func() {
			started := time.Now()
			status := "success"
			if err := runFileCleanup(files, locker, logger); err != nil {
				status = "error"
			}
			metrics.ObserveCron("file_upload_cleanup", status, started)
		}); err != nil {
			return nil, err
		}
	}
	if cfg.Cron.SampleSpec != "" {
		if _, err := runner.AddFunc(cfg.Cron.SampleSpec, func() {
			started := time.Now()
			status := "success"
			if err := runSample(locker, logger); err != nil {
				status = "error"
			}
			metrics.ObserveCron("sample", status, started)
		}); err != nil {
			return nil, err
		}
	}
	lc.Append(fx.Hook{OnStart: func(context.Context) error {
		if cfg.Cron.Enabled {
			runner.Start()
			logger.Info("scheduler started")
		}
		return nil
	}, OnStop: func(ctx context.Context) error {
		stopCtx := runner.Stop()
		select {
		case <-stopCtx.Done():
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}})
	return runner, nil
}

func runFileCleanup(files *filedomain.Service, locker *cache.Locker, logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var lock *cache.Lock
	if locker != nil {
		var acquired bool
		var err error
		lock, acquired, err = locker.TryLock(ctx, "cron:file-upload-cleanup", 11*time.Minute)
		if err != nil || !acquired {
			return err
		}
		defer func() {
			unlockCtx, unlockCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer unlockCancel()
			if err := lock.Unlock(unlockCtx); err != nil {
				logger.Error("file cleanup lock release failed", "error", err)
			}
		}()
	}
	count, err := files.CleanupExpiredUploads(ctx)
	logger.Info("expired upload cleanup completed", "count", count, "error", err)
	return err
}

func runSample(locker *cache.Locker, logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if locker == nil {
		logger.Info("sample scheduled job executed")
		return nil
	}
	lock, acquired, err := locker.TryLock(ctx, "cron:sample", time.Minute)
	if err != nil {
		logger.Error("scheduled job lock failed", "error", err)
		return err
	}
	if !acquired {
		logger.Info("sample scheduled job skipped", "reason", "lock held")
		return nil
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := lock.Unlock(unlockCtx); err != nil {
			logger.Error("scheduled job unlock failed", "error", err)
		}
	}()
	logger.Info("sample scheduled job executed")
	return nil
}

type slogWriter struct{ logger *slog.Logger }

func (w slogWriter) Printf(format string, args ...any) {
	w.logger.Error("scheduler event", "detail", format, "args", args)
}

var Module = fx.Module("scheduler", fx.Provide(New), fx.Invoke(func(*cron.Cron) {}))
