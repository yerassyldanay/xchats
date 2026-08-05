package whatsmeow

import (
	"fmt"
	"log/slog"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// slogBridge adapts an *slog.Logger to whatsmeow's own waLog.Logger
// interface (Warnf/Errorf/Infof/Debugf/Sub) — whatsmeow ships no slog
// adapter itself.
type slogBridge struct {
	log       *slog.Logger
	component string
}

// NewLogger wraps log for whatsmeow, tagging every line with a "component"
// attribute so per-account sub-loggers (via Sub) stay distinguishable in the
// shared log stream.
func NewLogger(log *slog.Logger, component string) waLog.Logger {
	return &slogBridge{log: log, component: component}
}

func (l *slogBridge) Warnf(msg string, args ...any) {
	l.log.Warn(fmt.Sprintf(msg, args...), "component", l.component)
}

func (l *slogBridge) Errorf(msg string, args ...any) {
	l.log.Error(fmt.Sprintf(msg, args...), "component", l.component)
}

func (l *slogBridge) Infof(msg string, args ...any) {
	l.log.Info(fmt.Sprintf(msg, args...), "component", l.component)
}

func (l *slogBridge) Debugf(msg string, args ...any) {
	l.log.Debug(fmt.Sprintf(msg, args...), "component", l.component)
}

func (l *slogBridge) Sub(module string) waLog.Logger {
	return &slogBridge{log: l.log, component: l.component + "/" + module}
}
