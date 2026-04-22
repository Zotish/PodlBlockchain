package zerolog

import (
	"fmt"
	"io"
	"os"
	"time"
)

type Level int8

const (
	Disabled Level = -1

	CallerFieldName = "caller"
)

type Logger struct {
	out   io.Writer
	level Level
}

type Context struct {
	logger Logger
}

type Event struct {
	logger Logger
}

type ConsoleWriter struct {
	Out        io.Writer
	TimeFormat string
}

func (w ConsoleWriter) Write(p []byte) (int, error) {
	if w.Out == nil {
		w.Out = os.Stdout
	}
	return w.Out.Write(p)
}

func New(w io.Writer) Logger {
	return Logger{out: w, level: 0}
}

func Nop() Logger {
	return Logger{out: io.Discard, level: Disabled}
}

func (l Logger) With() Context {
	return Context{logger: l}
}

func (l Logger) Output(w io.Writer) Logger {
	l.out = w
	return l
}

func (l Logger) GetLevel() Level {
	return l.level
}

func (l Logger) Debug() *Event    { return &Event{logger: l} }
func (l Logger) Info() *Event     { return &Event{logger: l} }
func (l Logger) Warn() *Event     { return &Event{logger: l} }
func (l Logger) Error() *Event    { return &Event{logger: l} }
func (l Logger) Fatal() *Event    { return &Event{logger: l} }
func (l Logger) Err(error) *Event { return &Event{logger: l} }

func (c Context) Timestamp() Context         { return c }
func (c Context) Str(string, string) Context { return c }
func (c Context) Int(string, int) Context    { return c }
func (c Context) Logger() Logger             { return c.logger }

func (e *Event) Str(string, string) *Event        { return e }
func (e *Event) Int(string, int) *Event           { return e }
func (e *Event) Int32(string, int32) *Event       { return e }
func (e *Event) Uint64(string, uint64) *Event     { return e }
func (e *Event) Dur(string, time.Duration) *Event { return e }
func (e *Event) Bool(string, bool) *Event         { return e }
func (e *Event) Float64(string, float64) *Event   { return e }
func (e *Event) Err(error) *Event                 { return e }
func (e *Event) Msg(msg string) {
	if e == nil || e.logger.out == nil || e.logger.out == io.Discard {
		return
	}
	_, _ = fmt.Fprintln(e.logger.out, msg)
}
func (e *Event) Msgf(format string, args ...any) {
	e.Msg(fmt.Sprintf(format, args...))
}
func (e *Event) Send() {
	if e != nil {
		e.Msg("")
	}
}
