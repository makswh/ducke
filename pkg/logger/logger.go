package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type LogEntry struct {
	ID        int64  `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`  // "INFO", "WARN", "ERROR", "DEBUG"
	Source    string `json:"source"` // e.g. "Steam", "Downloader", "Remote", "System", "Frontend"
	Message   string `json:"message"`
}

type Logger struct {
	mu          sync.RWMutex
	entries     []LogEntry
	maxEntries  int
	enabled     atomic.Bool
	nextID      atomic.Int64
	onEntryHook func(entry LogEntry)
	origOutput  io.Writer
}

var (
	globalLogger *Logger
	once         sync.Once
	datePrefixRe = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)?\s+`)
	tagPrefixRe  = regexp.MustCompile(`^\[([a-zA-Z0-9_\-\s]+)\]\s*`)
)

func InitLogger(maxEntries int) *Logger {
	once.Do(func() {
		if maxEntries <= 0 {
			maxEntries = 2000
		}
		globalLogger = &Logger{
			entries:    make([]LogEntry, 0, maxEntries),
			maxEntries: maxEntries,
			origOutput: os.Stdout,
		}
		globalLogger.enabled.Store(true)

		// Hook into standard Go log package
		log.SetOutput(globalLogger)
	})
	return globalLogger
}

func GetLogger() *Logger {
	if globalLogger == nil {
		return InitLogger(2000)
	}
	return globalLogger
}

func (l *Logger) SetEnabled(val bool) {
	l.enabled.Store(val)
}

func (l *Logger) IsEnabled() bool {
	return l.enabled.Load()
}

func (l *Logger) SetHook(hook func(entry LogEntry)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onEntryHook = hook
}

// Write implements io.Writer to intercept standard Go log.Printf / log.Println calls
func (l *Logger) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}

	// Always mirror to original stdout
	if l.origOutput != nil {
		_, _ = l.origOutput.Write(p)
	}

	// Remove standard log timestamp if present
	cleanMsg := datePrefixRe.ReplaceAllString(msg, "")

	// Extract source tag e.g. [Steam] or [Downloader]
	source := "System"
	if matches := tagPrefixRe.FindStringSubmatch(cleanMsg); len(matches) > 1 {
		source = strings.TrimSpace(matches[1])
		cleanMsg = strings.TrimSpace(cleanMsg[len(matches[0]):])
	}

	// Detect log level
	level := "INFO"
	lower := strings.ToLower(cleanMsg)
	if strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(lower, "fatal") {
		level = "ERROR"
	} else if strings.Contains(lower, "warn") || strings.Contains(lower, "rejected") {
		level = "WARN"
	} else if strings.Contains(lower, "debug") {
		level = "DEBUG"
	}

	l.AddEntry(level, source, cleanMsg)
	return len(p), nil
}

func (l *Logger) AddEntry(level, source, message string) LogEntry {
	entry := LogEntry{
		ID:        l.nextID.Add(1),
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Level:     strings.ToUpper(strings.TrimSpace(level)),
		Source:    strings.TrimSpace(source),
		Message:   strings.TrimSpace(message),
	}
	if entry.Level == "" {
		entry.Level = "INFO"
	}
	if entry.Source == "" {
		entry.Source = "System"
	}

	l.mu.Lock()
	if len(l.entries) >= l.maxEntries {
		// Drop oldest 10% to prevent continuous reallocations
		dropCount := l.maxEntries / 10
		if dropCount < 1 {
			dropCount = 1
		}
		l.entries = l.entries[dropCount:]
	}
	l.entries = append(l.entries, entry)
	hook := l.onEntryHook
	l.mu.Unlock()

	if hook != nil && l.enabled.Load() {
		hook(entry)
	}

	return entry
}

func (l *Logger) GetEntries() []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]LogEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = l.entries[:0]
}

func (l *Logger) GetFormattedText() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Ducke System Logs - Exported at %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("# Total Entries: %d\n\n", len(l.entries)))

	for _, entry := range l.entries {
		sb.WriteString(fmt.Sprintf("[%s] [%-5s] [%-10s] %s\n", entry.Timestamp, entry.Level, entry.Source, entry.Message))
	}

	return sb.String()
}

func (l *Logger) ExportToFile(filePath string) error {
	content := l.GetFormattedText()
	return os.WriteFile(filePath, []byte(content), 0644)
}
