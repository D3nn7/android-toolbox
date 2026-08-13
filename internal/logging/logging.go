// Package logging provides a simple file-backed logger and panic-recovery
// helpers so unexpected errors never crash the terminal outright.
package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
)

// Logger wraps the standard logger, writing to a file inside the app's log
// directory. It is safe to use a nil *Logger: all methods become no-ops.
type Logger struct {
	*log.Logger
	file *os.File
}

// New opens (creating if needed) the app log file inside logsDir.
func New(logsDir string) (*Logger, error) {
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(logsDir, "android-toolbox.log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Logger{
		Logger: log.New(f, "", log.LstdFlags|log.Lmicroseconds),
		file:   f,
	}, nil
}

// Close closes the underlying log file.
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

// Printf logs a formatted message, silently doing nothing if l is nil.
func (l *Logger) Printf(format string, v ...any) {
	if l == nil || l.Logger == nil {
		return
	}
	l.Logger.Printf(format, v...)
}

// Guard runs fn and recovers from any panic it raises, logging the panic
// with a full stack trace and returning a friendly error instead of letting
// the panic propagate and crash the process/terminal.
func (l *Logger) Guard(context string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			l.Printf("PANIC in %s: %v\n%s", context, r, stack)
			err = fmt.Errorf("internal error in %s: %v (see log for details)", context, r)
		}
	}()
	return fn()
}
