package main

import (
	"MrRSS/backend"

	"context"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetFeedList() []backend.FeedInfo {
	return backend.GetFeedList()
}

func (a *App) GetFeedContent() []backend.FeedContentFilterInfo {
	return backend.FilterFeedContent()
}

func (a *App) GetHistoryContent() []backend.FeedContentFilterInfo {
	return backend.GetHistoryContent()
}

func (a *App) WriteHistory(history []backend.FeedContentFilterInfo) error {
	return backend.WriteHistory(history)
}
