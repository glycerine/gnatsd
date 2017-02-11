package server

import (
	"net"
)

// InternalClient provides an interface
// to internal clients that live
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

	// Start should run the client on
	// a background goroutine.
	//
	// The Server s will invoke Start()
	// as a part of its own init and setup.
	//
	// The s pointer will be passed already locked,
	// and must not be unlocked during Start().
	//
	// The lsnReady channel
	// will be closed by the Server once the
	// accept loop has spun up.
	//
	// Before exiting, the client should
	// call back on register(nc)
	// to provide the server with
	// a net.Conn for communication.
	//
	// Any returned error will be logged and
	// will prevent the Server from calling
	// Stop() on termination. If nil is
	// returned then Stop() should function
	// correctly and not block.
	//
	Start(s *Server, lsnReady chan struct{}, register func(nc net.Conn)) error

	// Stop should shutdown the internal client.
	// The Server will invoke Stop() as a part
	// of its own shutdown process, so long as
	// Start() did not return an error.
	//
	Stop()
}
