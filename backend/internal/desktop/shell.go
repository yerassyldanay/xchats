//go:build desktop

package desktop

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/wailsapp/wails/v2"
	wailslogger "github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/version"
)

// Deps is everything the window needs from an already-booted backend. All of
// it is built by cmd/xchats' runServe and simply handed over — the shell
// constructs no application state of its own.
type Deps struct {
	// Router is internal/httpapi's gin engine, served same-origin to the
	// WebView through the asset server (see NewMiddleware).
	Router http.Handler
	// Hub is the realtime fan-out whose events become Wails events.
	Hub *realtime.Hub
	// Log is the application logger; Wails' own logging is routed into it so
	// a desktop run produces one stream, not two.
	Log *slog.Logger
	// Addr is the loopback address the backend also listens on, shown in the
	// startup log so a user can still reach the API from a browser or an MCP
	// client.
	Addr string
}

// Run opens the desktop window and blocks until the user closes it or ctx is
// cancelled (SIGINT/SIGTERM). It must be called on the main goroutine: macOS
// requires its UI event loop to own the main thread, and wails.Run is that
// loop.
//
// Returning hands control back to runServe's existing teardown — the same
// shutdown sequence a Ctrl-C on the server build runs.
func Run(ctx context.Context, d Deps) error {
	if d.Router == nil {
		return fmt.Errorf("desktop: no router to serve")
	}
	log := d.Log
	if log == nil {
		log = slog.Default()
	}

	// wailsCtx is only valid between OnStartup and OnShutdown. It is the
	// handle both the realtime pump (EventsEmit) and the ctx-cancelled path
	// (Quit) need, so it is captured once and read under a mutex rather than
	// passed around.
	var (
		mu        sync.Mutex
		wailsCtx  context.Context
		pumpStop  context.CancelFunc
		pumpDone  = make(chan struct{})
		pumpOnce  sync.Once
		closePump = func() { pumpOnce.Do(func() { close(pumpDone) }) }
	)

	app := &options.App{
		Title:     "xchats",
		Width:     1280,
		Height:    860,
		MinWidth:  1024,
		MinHeight: 640,
		// The SPA paints its own background immediately; matching it here
		// avoids a white flash on launch under a dark theme.
		BackgroundColour: options.NewRGB(255, 255, 255),
		AssetServer: &assetserver.Options{
			Assets:     Assets,
			Handler:    NewSPAHandler(Assets),
			Middleware: NewMiddleware(d.Router),
		},
		Logger:             wailsSlog{log},
		LogLevel:           wailslogger.INFO,
		LogLevelProduction: wailslogger.WARNING,
		// The store takes an exclusive file lock on the SQLite database
		// (internal/dbx.Open), so a second instance could not boot anyway —
		// it would die with "database is locked" instead of doing the thing
		// the user actually meant. The lock turns a second launch into
		// "focus the window that is already open".
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.xchats.desktop",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				mu.Lock()
				c := wailsCtx
				mu.Unlock()
				if c != nil {
					wailsruntime.WindowUnminimise(c)
					wailsruntime.Show(c)
				}
			},
		},
		Mac: &mac.Options{
			About: &mac.AboutInfo{
				Title:   "xchats",
				Message: "AI chat assistant for WhatsApp, Telegram, Instagram and Messenger.\nVersion " + version.Version,
			},
		},
		OnStartup: func(c context.Context) {
			pctx, cancel := context.WithCancel(ctx)
			mu.Lock()
			wailsCtx, pumpStop = c, cancel
			mu.Unlock()
			log.Info("desktop window ready", "version", version.Version, "backend", "http://"+d.Addr)
			go func() {
				defer closePump()
				// The realtime layer's desktop transport: hub events become
				// Wails events, which frontend/src/lib/sse.ts binds exactly
				// as it binds the SSE stream in a browser.
				PumpRealtime(pctx, d.Hub, func(name string, data any) {
					wailsruntime.EventsEmit(c, name, data)
				})
			}()
		},
		OnShutdown: func(context.Context) {
			mu.Lock()
			stop := pumpStop
			wailsCtx, pumpStop = nil, nil
			mu.Unlock()
			if stop != nil {
				stop()
				<-pumpDone
			}
		},
	}

	// A Ctrl-C in the terminal (or a SIGTERM from a session manager) has to
	// close the window too — otherwise the process would sit in wails.Run
	// with a cancelled context and every worker already winding down.
	go func() {
		<-ctx.Done()
		mu.Lock()
		c := wailsCtx
		mu.Unlock()
		if c == nil {
			return
		}
		log.Info("shutdown signal received; closing the desktop window")
		wailsruntime.Quit(c)
	}()

	err := wails.Run(app)
	log.Info("desktop window closed")
	closePump()
	return err
}

// wailsSlog adapts Wails' logger to the application's own slog handler, so a
// desktop run emits one log stream in the format config.yaml asked for
// instead of Wails' separate stdout writer.
type wailsSlog struct{ log *slog.Logger }

func (w wailsSlog) Print(m string)   { w.log.Info(m, "src", "wails") }
func (w wailsSlog) Trace(m string)   { w.log.Debug(m, "src", "wails") }
func (w wailsSlog) Debug(m string)   { w.log.Debug(m, "src", "wails") }
func (w wailsSlog) Info(m string)    { w.log.Info(m, "src", "wails") }
func (w wailsSlog) Warning(m string) { w.log.Warn(m, "src", "wails") }
func (w wailsSlog) Error(m string)   { w.log.Error(m, "src", "wails") }
func (w wailsSlog) Fatal(m string)   { w.log.Error(m, "src", "wails", "fatal", true) }
