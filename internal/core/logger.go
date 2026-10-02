package core

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

var (
	stdoutWriter io.Writer = os.Stdout
	stderrWriter io.Writer = os.Stderr
)

// FormatLogLine formats a timestamp, level, component and message into the standard format:
// [YYYY-MM-DD HH:MM:SS] [LEVEL] [COMPONENT] Message
func FormatLogLine(t time.Time, level, component, message string) string {
	if component == "" {
		component = "CORE"
	}
	return fmt.Sprintf("[%s] [%s] [%s] %s",
		t.Format("2006-01-02 15:04:05"),
		strings.ToUpper(level),
		strings.ToUpper(component),
		message,
	)
}

// LogEmitter handles emitting formatted log lines to appropriate streams.
type LogEmitter struct {
	Stdout io.Writer
	Stderr io.Writer
}

// NewLogEmitter creates a new LogEmitter with the given writers.
func NewLogEmitter(stdout, stderr io.Writer) *LogEmitter {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	return &LogEmitter{
		Stdout: stdout,
		Stderr: stderr,
	}
}

// Emit formats and writes a log entry according to its severity level.
func (e *LogEmitter) Emit(t time.Time, level, component, message string) {
	line := FormatLogLine(t, level, component, message)
	upperLevel := strings.ToUpper(level)
	if upperLevel == "WARN" || upperLevel == "ERROR" {
		fmt.Fprintln(e.Stderr, line)
	} else {
		fmt.Fprintln(e.Stdout, line)
	}
}

// WriteLogLine writes a log line to stdout (INFO/DEBUG) or stderr (WARN/ERROR).
func WriteLogLine(t time.Time, level, component, message string) {
	line := FormatLogLine(t, level, component, message)
	upperLevel := strings.ToUpper(level)
	if upperLevel == "WARN" || upperLevel == "ERROR" {
		fmt.Fprintln(stderrWriter, line)
	} else {
		fmt.Fprintln(stdoutWriter, line)
	}
}
