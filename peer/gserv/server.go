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
	peer api.LocalGetSet
}

func NewPeerServerClass(peer api.LocalGetSet) *PeerServerClass {
	return &PeerServerClass{
		peer: peer,
	}
}

// implement pb.PeerServer interface
func (s *PeerServerClass) SendFile(stream pb.Peer_SendFileServer) error {
	p("peer.Server SendFile starting!")
	var chunkCount int64
	path := ""
	var bc int64
	var hasher hash.Hash
	var err error

	hasher, err = blake2b.New(nil)
	if err != nil {
		return err
	}

	var finalChecksum []byte
	var fd *os.File
	bytesSeen := 0

	defer func() {
		if fd != nil {
			fd.Close()
		}
		p("this server.SendFile() call got %v chunks, byteCount=%v. with "+
			"final checksum '%x'", chunkCount, bytesSeen, finalChecksum)
	}()

	firstChunkSeen := false
	var allChunks bytes.Buffer

	for {
		nk, err := stream.Recv()
		if err == io.EOF {
			if nk != nil && len(nk.Data) > 0 {
				// we are assuming that this never happens!
				panic("we need to save this last chunk too!")
			}
			p("server doing stream.Recv(); sees err == "+
				"io.EOF. nk=%p. bytesSeen=%v. chunkCount=%v.",
				nk, bytesSeen, chunkCount)

			finalChecksum = []byte(hasher.Sum(nil))
			endTime := time.Now()
			return stream.SendAndClose(&pb.BigFileAck{
				Filepath:         path,
				SizeInBytes:      bc,
				RecvTime:         uint64(endTime.UnixNano()),
				WholeFileBlake2B: finalChecksum,
			})
		}
		if err != nil {
			return err
		}
		bytesSeen += len(nk.Data)

		// INVAR: we have a chunk
		if !firstChunkSeen {
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
			p("cumulative checksum on nk.ChunkNumber=%v looks good; cumul='%x'.  nk.IsLastChunk=%v", nk.ChunkNumber, nk.Blake2BCumulative, nk.IsLastChunk)
		}
		if path == "" {
			path = nk.Filepath
			p("peer.Server SendFile sees new file '%s'", path)
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
		bc += nk.SizeInBytes
		chunkCount++

		nw, err := allChunks.Write(nk.Data)
		panicOn(err)
		if nw != len(nk.Data) {
			panic("short write to bytes.Buffer")
		}

		if nk.IsLastChunk {
			toWrite := allChunks.Bytes()
			err = s.peer.LocalSet(&api.KeyInv{Key: []byte("chk"), Val: toWrite, When: time.Unix(0, int64(nk.SendTime))})
			p("server sees the last chunk of '%s', writing to bolt %v bytes gave '%v', and returning now.", nk.Filepath, len(toWrite), err)
		}

	} // end for
	return nil
}

const ProgramName = "gprcServer"

func MainExample() {

	myflags := flag.NewFlagSet(ProgramName, flag.ExitOnError)
	cfg := &ServerConfig{}
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
	if cfg != nil && cfg.GrpcServer != nil {
		cfg.GrpcServer.Stop()
	}
}

func (cfg *ServerConfig) StartGrpcServer(peer api.LocalGetSet, sshdReady chan bool) {

	var gRpcBindPort int
	var gRpcHost string
	if cfg.UseTLS {
		// use TLS
		gRpcBindPort = cfg.ExternalLsnPort
		gRpcHost = cfg.Host

		p("gRPC with TLS listening on %v:%v", gRpcHost, gRpcBindPort)

	} else {
		// SSH will take the external, gRPC will take the internal.
		gRpcBindPort = cfg.InternalLsnPort
		gRpcHost = "127.0.0.1" // local only, behind the SSHD

		p("external SSHd listening on %v:%v, internal gRPC service listening on 127.0.0.1:%v", cfg.Host, cfg.ExternalLsnPort, cfg.InternalLsnPort)

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

	cfg.GrpcServer = grpc.NewServer(opts...)
	pb.RegisterPeerServer(cfg.GrpcServer, NewPeerServerClass(peer))

	// blocks until shutdown
	cfg.GrpcServer.Serve(lis)
}

func blake2bOfBytes(by []byte) []byte {
	h, err := blake2b.New(nil)
	panicOn(err)
	h.Write(by)
	return []byte(h.Sum(nil))
}