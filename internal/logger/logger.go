// Package logger provides a thread-safe in-memory and file based journal
// shared by every stage of the conversion pipeline.
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Level string

const (
	LevelDebug   Level = "DEBUG"
	LevelInfo    Level = "INFO"
	LevelWarning Level = "WARN"
	LevelError   Level = "ERROR"
)

type Entry struct {
	Time    time.Time
	Level   Level
	Message string
}

func (e Entry) String() string {
	return fmt.Sprintf("[%s] %-5s %s", e.Time.Format("2006-01-02 15:04:05"), e.Level, e.Message)
}

type Observer func(Entry)

type Logger struct {
	mu        sync.RWMutex
	entries   []Entry
	observers []Observer
	file      *os.File
}

// New creates a logger that only keeps entries in memory.
func New() *Logger {
	return &Logger{}
}

// NewWithFile creates a logger that also appends every entry to path.
func NewWithFile(path string) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Logger{file: f}, nil
}

// DefaultPath returns the standard journal location inside the user cache.
func DefaultPath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "kuznica", "kuznica.log")
}

func (l *Logger) Subscribe(o Observer) {
	l.mu.Lock()
	l.observers = append(l.observers, o)
	l.mu.Unlock()
}

func (l *Logger) log(level Level, format string, args ...any) {
	entry := Entry{Time: time.Now(), Level: level, Message: fmt.Sprintf(format, args...)}

	l.mu.Lock()
	l.entries = append(l.entries, entry)
	observers := make([]Observer, len(l.observers))
	copy(observers, l.observers)
	if l.file != nil {
		fmt.Fprintln(l.file, entry.String())
	}
	l.mu.Unlock()

	for _, o := range observers {
		o(entry)
	}
}

func (l *Logger) Debugf(format string, args ...any)   { l.log(LevelDebug, format, args...) }
func (l *Logger) Infof(format string, args ...any)    { l.log(LevelInfo, format, args...) }
func (l *Logger) Warningf(format string, args ...any) { l.log(LevelWarning, format, args...) }
func (l *Logger) Errorf(format string, args ...any)   { l.log(LevelError, format, args...) }

// Entries returns a copy of the journal.
func (l *Logger) Entries() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}

// Text renders the whole journal as plain text.
func (l *Logger) Text() string {
	var buf []byte
	for _, e := range l.Entries() {
		buf = append(buf, e.String()...)
		buf = append(buf, '\n')
	}
	return string(buf)
}

func (l *Logger) Clear() {
	l.mu.Lock()
	l.entries = nil
	l.mu.Unlock()
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}
