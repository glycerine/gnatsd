package server

import (
	"github.com/nats-io/gnatsd/util/lcon"
)

// reply to cluster membership queries.
// contains a local client that communicates
// over a pipe.
type groupmember struct {
	client *client
	pipe   *lcon.Pipe
}

func (s *Server) CreateInternalMembershipClient(
	maxpay int,
	clientListenReady chan struct{},
) {

	go func() {
		gm := &groupmember{
			pipe: lcon.NewPipe(make([]byte, maxpay)),
		}
		// startup phase
		select {
		case <-clientListenReady:
			gm.client = s.createClient(gm.pipe)
			s.mu.Lock()
			s.membership = gm
			s.mu.Unlock()
		case <-s.rcQuit:
		}

		/*
			// service requests until shutdown
			req := make([]byte, maxpay)
			for {
				select {
				case <-s.rcQuit:
					return
				case <-gm.pipe.Flushed:
					nr, err := gm.pipe.Read(req)
					if err != nil {
						panic(err)
					}
					see := req[:nr]

				}
			}
		*/
	}()
}
