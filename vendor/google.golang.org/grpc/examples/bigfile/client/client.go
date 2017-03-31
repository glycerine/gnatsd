package main

import (
	"bytes"
	"flag"
	"hash"
	"time"

	"github.com/glycerine/blake2b" // vendor https://github.com/dchest/blake2b"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	pb "google.golang.org/grpc/examples/bigfile/streambigfile"
	"google.golang.org/grpc/grpclog"
)

var (
	tls                = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	caFile             = flag.String("ca_file", "../testdata/ca.pem", "The file containning the CA root cert file")
	serverAddr         = flag.String("server_addr", "127.0.0.1:10000", "The server address in the format of host:port")
	serverHostOverride = flag.String("server_host_override", "x.test.youtube.com", "The server name use to verify the hostname returned by TLS handshake")
)

type client struct {
	hasher hash.Hash
}

func newClient() *client {
	h, err := blake2b.New(nil)
	panicOn(err)
	return &client{
		hasher: h,
	}
}

func (c *client) runSendFile(client pb.PeerClient, data []byte) {
	// Create a random number of random points

	stream, err := client.SendFile(context.Background())
	if err != nil {
		grpclog.Fatalf("%v.SendFile(_) = _, %v", client, err)
	}
	var nk pb.BigFileChunk
	nk.Filepath = "hello"
	nk.SizeInBytes = int64(len(data))
	nk.SendTime = uint64(time.Now().UnixNano())
	nk.Blake2B = blake2bOfBytes(data)
	nk.Data = data

	c.hasher.Write(nk.Data)

	if err := stream.Send(&nk); err != nil {
		grpclog.Fatalf("%v.Send(%v) = %v", stream, nk, err)
	}

	reply, err := stream.CloseAndRecv()
	if err != nil {
		grpclog.Fatalf("%v.CloseAndRecv() got error %v, want %v", stream, err, nil)
	}
	compared := bytes.Compare(reply.WholeFileBlake2B, []byte(c.hasher.Sum(nil)))
	grpclog.Printf("Reply saw checksum: '%x' match: %v", reply.WholeFileBlake2B, compared == 0)
}

func main() {
	flag.Parse()
	var opts []grpc.DialOption
	if *tls {
		var sn string
		if *serverHostOverride != "" {
			sn = *serverHostOverride
		}
		var creds credentials.TransportCredentials
		if *caFile != "" {
			var err error
			creds, err = credentials.NewClientTLSFromFile(*caFile, sn)
			if err != nil {
				grpclog.Fatalf("Failed to create TLS credentials %v", err)
			}
		} else {
			creds = credentials.NewClientTLSFromCert(nil, sn)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		grpclog.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewPeerClient(conn)

	// SendFile
	data := []byte("hello peer")
	c := newClient()
	c.runSendFile(client, data)
}

func blake2bOfBytes(by []byte) []byte {
	h, err := blake2b.New(nil)
	panicOn(err)
	h.Write(by)
	return []byte(h.Sum(nil))
}
