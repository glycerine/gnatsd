// Copyright 2012-2015 Apcera Inc. All rights reserved.

//Package logger provides logging facilities for the NATS server
package logger

import (
	"log"
)

// StdLogLogger assists with embedding gnatsd by
// providing an implementation of the interface
// need to call server.SetLogger() and still log
// to the Go standard library's log.
type StdLogLogger struct {
	debug      bool
	trace      bool
	infoLabel  string
	errorLabel string
	fatalLabel string
	debugLabel string
	traceLabel string
}

// NewStdLogLogger sends log output to the standard lib log.std Logger
func NewStdLogLogger(time, debug, trace, colors, pid bool, flags int) *StdLogLogger {
	if time {
		flags = log.LstdFlags | log.Lmicroseconds | log.LUTC
	}

	pre := ""
	if pid {
		pre = pidPrefix()
	}
	log.SetPrefix(pre)

	l := &StdLogLogger{
		debug: debug,
		trace: trace,
	}

	// colors ignored, always do plain.
	l.infoLabel = "[INF] "
	l.debugLabel = "[DBG] "
	l.errorLabel = "[ERR] "
	l.fatalLabel = "[FTL] "
	l.traceLabel = "[TRC] "

	return l
}

// Noticef logs a notice statement
func (l *StdLogLogger) Noticef(format string, v ...interface{}) {
	log.Printf(l.infoLabel+format, v...)
}

// Errorf logs an error statement
func (l *StdLogLogger) Errorf(format string, v ...interface{}) {
	log.Printf(l.errorLabel+format, v...)
}

// Fatalf logs a fatal error
func (l *StdLogLogger) Fatalf(format string, v ...interface{}) {
	log.Fatalf(l.fatalLabel+format, v...)
}

// Debugf logs a debug statement
func (l *StdLogLogger) Debugf(format string, v ...interface{}) {
	if l.debug {
		log.Printf(l.debugLabel+format, v...)
	}
}

// Tracef logs a trace statement
func (l *StdLogLogger) Tracef(format string, v ...interface{}) {
	if l.trace {
		log.Printf(l.traceLabel+format, v...)
	}
}
