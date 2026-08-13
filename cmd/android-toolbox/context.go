package main

import (
	"context"
	"os"

	"android-toolbox/internal/config"
	"android-toolbox/internal/logging"
	"android-toolbox/internal/selfupdate"
)

type ctxKey struct{}

// appContext bundles everything most commands need: resolved paths, loaded
// settings/state, and the file logger.
type appContext struct {
	Paths    config.Paths
	Settings config.Settings
	State    config.State
	Log      *logging.Logger
}

func withAppContext(ctx context.Context, a *appContext) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

func appContextFrom(ctx context.Context) *appContext {
	a, _ := ctx.Value(ctxKey{}).(*appContext)
	return a
}

func newAppContext() (*appContext, error) {
	// Best-effort: removes a ".old" binary left behind by a previous
	// `self-update` (see internal/selfupdate.Apply) now that it's no longer
	// the running executable and Windows will actually let it be deleted.
	if exePath, err := os.Executable(); err == nil {
		selfupdate.CleanupOldBinary(exePath)
	}

	paths, err := config.Resolve()
	if err != nil {
		return nil, err
	}

	logger, err := logging.New(paths.LogsDir)
	if err != nil {
		return nil, err
	}

	settings, err := config.LoadSettings(paths)
	if err != nil {
		logger.Printf("failed to load settings: %v", err)
		return nil, err
	}

	state, err := config.LoadState(paths)
	if err != nil {
		logger.Printf("failed to load state: %v", err)
		return nil, err
	}

	return &appContext{
		Paths:    paths,
		Settings: settings,
		State:    state,
		Log:      logger,
	}, nil
}
