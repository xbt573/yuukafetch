package logger

import (
	"fmt"
	"log/slog"
)

var (
	_verbose bool
	_quiet   bool
)

func Initialize(verbose, quiet bool) {
	_verbose = verbose
	_quiet = quiet
}

func Println(msg string) {
	Printf("%s", msg)
}

func Printf(format string, args ...any) {
	if _verbose || _quiet {
		return
	}

	fmt.Printf(format, args...)
}

func Log(msg string) {
	if !_verbose && !_quiet {
		return
	}

	slog.Info(msg)
}

func Logf(format string, args ...any) {
	if !_verbose && !_quiet {
		return
	}

	Log(fmt.Sprintf(format, args...))
}

func Error(msg string) {
	slog.Error(msg)
}

func Errorf(format string, args ...any) {
	Error(fmt.Sprintf(format, args...))
}
