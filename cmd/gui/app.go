package main

import (
	"context"

	"github.com/MutsiMutsi/go-novon/core"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx      context.Context
	streamer *core.Streamer
}

// App struct
type ClientCreated struct {
	Address string
	Wallet  string
	Error   error
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

func (a *App) StartStream() ClientCreated {
	if a.streamer != nil && a.streamer.IsActive() {
		return ClientCreated{
			Address: a.streamer.ClientAddress(),
			Wallet:  a.streamer.ClientWalletAddress(),
			Error:   nil,
		}
	}

	a.streamer = core.NewStreamer()

	a.streamer.EventHandler.Subscribe(func(data interface{}) {
		runtime.EventsEmit(a.ctx, "UpdateEvent", data)
	})

	err := a.streamer.Start()

	return ClientCreated{
		Address: a.streamer.ClientAddress(),
		Wallet:  a.streamer.ClientWalletAddress(),
		Error:   err,
	}
}

func (a *App) StopStream() {
	if a.streamer != nil {
		a.streamer.Stop()
	}
}

func (a *App) IsStreaming() bool {
	return a.streamer != nil && a.streamer.IsActive()
}
