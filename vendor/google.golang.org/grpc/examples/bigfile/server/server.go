package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"time"

	"google.golang.org/grpc"

	"github.com/glycerine/blake2b" // vendor https://github.com/dchest/blake2b"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	pb "google.golang.org/grpc/examples/bigfile/streambigfile"
)

var (
	tls      = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	certFile = flag.String("cert_file", "../testdata/server1.pem", "The TLS cert file")
	keyFile  = flag.String("key_file", "../testdata/server1.key", "The TLS key file")
	port     = flag.Int("port", 10000, "The server port")
)

type peerServer struct {
	chunks []*pb.BigFileChunk
}

// implement pb.PeerServer interface
func (s *peerServer) SendFile(stream pb.Peer_SendFileServer) error {
	p("SendFile starting!")
	var chunkCount int64
	path := ""
	var bc int64

	hasher, err := blake2b.New(nil)
	panicOn(err)
	var finalChecksum []byte

	defer func() {
		fmt.Printf("\n this server.SendFile() call got %v chunks, with "+
			"final checksum '%x'\n", chunkCount, finalChecksum)
	}()

	for {
		nk, err := stream.Recv()
		if err == io.EOF {
			finalChecksum = []byte(hasher.Sum(nil))
			endTime := time.Now()
			return stream.SendAndClose(&pb.BigFileAck{
				Filepath:         path,
				RecvTime:         uint64(endTime.UnixNano()),
				WholeFileBlake2B: finalChecksum,
			})
		}
		if err != nil {
			return err
		}
		hasher.Write(nk.Data)

		if path == "" {
			path = nk.Filepath
		}
		bc += nk.SizeInBytes
		if nk.SizeInBytes != int64(len(nk.Data)) {
			return fmt.Errorf("%v == nk.SizeInBytes != int64(len(nk.Data)) == %v", nk.SizeInBytes, int64(len(nk.Data)))
		}
		chunkCount++
		s.chunks = append(s.chunks, nk)
	}
	return nil
}

func newPeerServer() *peerServer {
	s := new(peerServer)
	return s
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		grpclog.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	if *tls {
		creds, err := credentials.NewServerTLSFromFile(*certFile, *keyFile)
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterPeerServer(grpcServer, newPeerServer())
	p("listening on %v", *port)
	grpcServer.Serve(lis)
}
