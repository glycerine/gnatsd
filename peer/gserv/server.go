// gRPC server
package gserv

import (
	"bytes"
	"flag"
	"fmt"
	"hash"
	"io"
	"log"
	"net"
	"os"
	"runtime/pprof"
	"time"

	"google.golang.org/grpc"

	"github.com/glycerine/blake2b" // vendor https://github.com/dchest/blake2b"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	"github.com/glycerine/hnatsd/peer/api"
	pb "github.com/glycerine/hnatsd/peer/streambigfile"
)

type PeerServerClass struct {
	lgs api.LocalGetSet
	cfg *ServerConfig
}

func NewPeerServerClass(lgs api.LocalGetSet, cfg *ServerConfig) *PeerServerClass {
	return &PeerServerClass{
		lgs: lgs,
		cfg: cfg,
	}
}

// implement pb.PeerServer interface; the server is receiving a file here,
//  because the client called SendFile() on the other end.
//
func (s *PeerServerClass) SendFile(stream pb.Peer_SendFileServer) (err error) {
	//p("%s peer.Server SendFile starting!", s.cfg.MyID)
	var chunkCount int64
	path := ""
	var hasher hash.Hash

	hasher, err = blake2b.New(nil)
	if err != nil {
		return err
	}

	var finalChecksum []byte
	var fd *os.File
	var bytesSeen int64
	var isBcastSet bool

	defer func() {
		if fd != nil {
			fd.Close()
		}
		finalChecksum = []byte(hasher.Sum(nil))
		endTime := time.Now()

		//p("%s this server.SendFile() call got %v chunks, byteCount=%v. with final checksum '%x'. defer running/is returning with err='%v'", s.cfg.MyID, chunkCount, bytesSeen, finalChecksum, err)
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}

		sacErr := stream.SendAndClose(&pb.BigFileAck{
			Filepath:         path,
			SizeInBytes:      bytesSeen,
			RecvTime:         uint64(endTime.UnixNano()),
			WholeFileBlake2B: finalChecksum,
			Err:              errStr,
		})
		if sacErr != nil {
			log.Printf("warning: sacErr='%s' in gserv server.go PeerServerClass.SendFile() attempt to stream.SendAndClose().", sacErr)
		}
	}()

	firstChunkSeen := false
	var allChunks bytes.Buffer
	var nk *pb.BigFileChunk

	for {
		nk, err = stream.Recv()
		if err == io.EOF {
			if nk != nil && len(nk.Data) > 0 {
				// we are assuming that this never happens!
				panic("we need to save this last chunk too!")
			}
			//p("server doing stream.Recv(); sees err == io.EOF. nk=%p. bytesSeen=%v. chunkCount=%v.", nk, bytesSeen, chunkCount)

			return nil
		}
		if err != nil {
			return err
		}

		// INVAR: we have a chunk
		if !firstChunkSeen {
			isBcastSet = nk.IsBcastSet
			if nk.Filepath != "" {
				fd, err = os.Create(nk.Filepath + fmt.Sprintf("__%v", time.Now()))
				if err != nil {
					return err
				}
			}
			firstChunkSeen = true
		}

		hasher.Write(nk.Data)
		cumul := []byte(hasher.Sum(nil))
		if 0 != bytes.Compare(cumul, nk.Blake2BCumulative) {
			return fmt.Errorf("cumulative checksums failed at chunk %v of '%s'. Observed: '%x', expected: '%x'.", nk.ChunkNumber, nk.Filepath, cumul, nk.Blake2BCumulative)
		} else {
			//p("cumulative checksum on nk.ChunkNumber=%v looks good; cumul='%x'.  nk.IsLastChunk=%v", nk.ChunkNumber, nk.Blake2BCumulative, nk.IsLastChunk)
		}
		if path == "" {
			path = nk.Filepath
			//p("peer.Server SendFile sees new file '%s'", path)
		}
		if path != "" && path != nk.Filepath {
			panic(fmt.Errorf("confusing between two different streams! '%s' vs '%s'", path, nk.Filepath))
		}

		if nk.SizeInBytes != int64(len(nk.Data)) {
			return fmt.Errorf("%v == nk.SizeInBytes != int64(len(nk.Data)) == %v", nk.SizeInBytes, int64(len(nk.Data)))
		}

		checksum := blake2bOfBytes(nk.Data)
		cmp := bytes.Compare(checksum, nk.Blake2B)
		if cmp != 0 {
			return fmt.Errorf("chunk %v bad .Data, checksum mismatch!",
				nk.ChunkNumber)
		}

		// INVAR: chunk passes tests, keep it.
		bytesSeen += int64(len(nk.Data))
		chunkCount++
		var nw int

		nw, err = allChunks.Write(nk.Data)
		panicOn(err)
		if nw != len(nk.Data) {
			panic("short write to bytes.Buffer")
		}

		if nk.IsLastChunk {
			kiBytes := allChunks.Bytes()

			if isBcastSet {
				// is a BcastSet request
				var req api.BcastSetRequest
				_, err = req.UnmarshalMsg(kiBytes)
				if err != nil {
					return fmt.Errorf("gserv/server.go SendFile(): req.UnmarshalMsg() errored '%v'", err)
				}

				//p("%s this server.SendFile() with BcastSet, got request from '%s' to set key '%s' with data of len %v. debug Val:'%s'. about to wait on chan %p", s.cfg.MyID, req.FromID, req.Ki.Key, len(req.Ki.Val), string(req.Ki.Val[:intMin(100, len(req.Ki.Val))]), s.cfg.ServerGotSetRequest)

				// notify peer by sending on cfg.ServerGotSetRequest
				select {
				case s.cfg.ServerGotSetRequest <- &req:
					//p("gserv server.go  sent BcastSetRequest on channel s.cfg.ServerGotSetRequest")
				case <-s.cfg.Halt.ReqStop.Chan:
				}

			} else {
				// is a BcastGet reply
				var reply api.BcastGetReply
				_, err = reply.UnmarshalMsg(kiBytes)
				if err != nil {
					return fmt.Errorf("gserv/server.go SendFile(): reply.UnmarshalMsg() errored '%v'", err)
				}

				//p("%s this server.SendFile() with BcastGet, got request from '%s' to get key '%s' with data of len %v. debug Val:'%s'. about to send on chan %p", s.cfg.MyID, reply.FromID, reply.Ki.Key, len(reply.Ki.Val), string(reply.Ki.Val[:intMin(100, len(reply.Ki.Val))]), s.cfg.ServerGotGetReply)

				// notify peer by sending on cfg.ServerGotGetReply
				select {
				case s.cfg.ServerGotGetReply <- &reply:
					//p("gserv server.go sent BcastGetReply on channel s.cfg.ServerGotReply")
				case <-s.cfg.Halt.ReqStop.Chan:
				}
			}
			return err
		}

	} // end for
	return nil
}

const ProgramName = "gprcServer"

func MainExample() {

	myflags := flag.NewFlagSet(ProgramName, flag.ExitOnError)
	myID := "123"
	cfg := NewServerConfig(myID)
	cfg.DefineFlags(myflags)

	sshegoCfg := setupSshFlags(myflags)

	err := myflags.Parse(os.Args[1:])

	if cfg.CpuProfilePath != "" {
		f, err := os.Create(cfg.CpuProfilePath)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	err = cfg.ValidateConfig()
	if err != nil {
		log.Fatalf("%s command line flag error: '%s'", ProgramName, err)
	}
	cfg.SshegoCfg = sshegoCfg
	//	cfg.StartGrpcServer()
}

func (cfg *ServerConfig) Stop() {
	if cfg != nil {
		cfg.mut.Lock()
		if cfg.GrpcServer != nil {
			cfg.GrpcServer.Stop() // race here in Test103BcastGet
		}
		cfg.mut.Unlock()
	}
}

func (cfg *ServerConfig) StartGrpcServer(
	peer api.LocalGetSet,
	sshdReady chan bool,
	myID string,
) {

	var gRpcBindPort int
	var gRpcHost string
	if cfg.UseTLS {
		// use TLS
		gRpcBindPort = cfg.ExternalLsnPort
		gRpcHost = cfg.Host

		//p("gRPC with TLS listening on %v:%v", gRpcHost, gRpcBindPort)

	} else {
		// SSH will take the external, gRPC will take the internal.
		gRpcBindPort = cfg.InternalLsnPort
		gRpcHost = "127.0.0.1" // local only, behind the SSHD

		//p("%s external SSHd listening on %v:%v, internal gRPC service listening on 127.0.0.1:%v", myID, cfg.Host, cfg.ExternalLsnPort, cfg.InternalLsnPort)

	}

	lis, err := net.Listen("tcp", fmt.Sprintf("%v:%d", gRpcHost, gRpcBindPort))
	if err != nil {
		grpclog.Fatalf("failed to listen: %v", err)
	}

	var opts []grpc.ServerOption

	if cfg.UseTLS {
		// use TLS
		creds, err := credentials.NewServerTLSFromFile(cfg.CertPath, cfg.KeyPath)
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	} else {
		// use SSH
		err = serverSshMain(cfg.SshegoCfg, cfg.Host,
			cfg.ExternalLsnPort, cfg.InternalLsnPort)
		panicOn(err)
		close(sshdReady)
	}

	cfg.mut.Lock()
	cfg.GrpcServer = grpc.NewServer(opts...) // race here, prev write, conflicts 226 line. in Test103BcastGet observed with go test -race
	cfg.mut.Unlock()
	pb.RegisterPeerServer(cfg.GrpcServer, NewPeerServerClass(peer, cfg))

	// blocks until shutdown
	cfg.GrpcServer.Serve(lis)
}

func blake2bOfBytes(by []byte) []byte {
	h, err := blake2b.New(nil)
	panicOn(err)
	h.Write(by)
	return []byte(h.Sum(nil))
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
