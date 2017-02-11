package server

import (
	"net"
)

// iCli tracks which internal
// clients were succesfully Start()-ed
// in the running slice. The
// server notice if Start() returned
// an error, and will exclude that configured
// client from the running list.
// Only the running list will have
// Stop() called on them at shutdown.
//
type iCli struct {
	configured []InternalClient
	running    []InternalClient
}

// InternalClient provides
// a plugin-like interface,
// supporting internal clients that live
// in-process with the Server
// on their own goroutines.
//
// An example of an internal client
// is the health monitoring client.
// In order to be effective, its lifetime
// must exactly match that of the
// server it monitors.
//
type InternalClient interface {

	// Name should return a readable
	// human name for the InternalClient;
	// it will be invoked as a part of
	// startup/shutdown/error logging.
	//
	Name() string

	// Start should run the client on
	// a background goroutine.
	//
	// The Server s will invoke Start()
	// as a part of its own init and setup.
	//
	// The info and opts pointers will be
	// viewable from an already locked Server
	// instance, and so can be read without
	// worrying about data races.
	//
	// The client should wait on the
	// lsnReady channel in a background
	// goroutine. The lsnReady channel
	// will be closed by the Server once the
	// accept loop has spun up. Only
	// once that has happened should
	// the client call accept(nc).
	//
	// By calling accept(nc), the client
	// provides the server with the
	// equivalent of a Listen/Accept created
	// net.Conn for communication.
	//
	// Any returned error will be logged and
	// will prevent the Server from calling
	// Stop() on termination. If nil is
	// returned then Stop() should be
	// expected.
	//
	Start(info Info,
		opts Options,
		lsnReady chan struct{},
		accept func(nc net.Conn)) error

	// Stop should shutdown the goroutine(s)
	// of the internal client.
	// The Server will invoke Stop() as a part
	// of its own shutdown process, so long as
	// Start() did not return an error.
	//
	// Stop is expected not to block for long.
	//
	Stop()
}
