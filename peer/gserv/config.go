package gserv

import (
	"flag"
	"fmt"
	"net"

	"github.com/glycerine/hnatsd/peer/api"
	tun "github.com/glycerine/sshego"
	"google.golang.org/grpc"
)

type ServerConfig struct {
	Host string // ip address

	// by default we use SSH
	UseTLS bool

	CertPath string
	KeyPath  string

	ExternalLsnPort int
	InternalLsnPort int
	CpuProfilePath  string

	SshegoCfg *tun.SshegoConfig

	ServerGotReply chan *api.BcastGetReply

	GrpcServer *grpc.Server
}

func NewServerConfig() *ServerConfig {
	return &ServerConfig{
		ServerGotReply: make(chan *api.BcastGetReply),
	}
}

func (c *ServerConfig) DefineFlags(fs *flag.FlagSet) {
	fs.BoolVar(&c.UseTLS, "tls", false, "Use TLS instead of the default SSH.")
	fs.StringVar(&c.CertPath, "cert_file", "testdata/server1.pem", "The TLS cert file")
	fs.StringVar(&c.KeyPath, "key_file", "testdata/server1.key", "The TLS key file")
	fs.StringVar(&c.Host, "host", "127.0.0.1", "host IP address or name to bind")
	fs.IntVar(&c.ExternalLsnPort, "externalport", 10000, "The exteral server port")
	fs.IntVar(&c.InternalLsnPort, "iport", 10001, "The internal server port")
	fs.StringVar(&c.CpuProfilePath, "cpuprofile", "", "write cpu profile to file")
}

func (c *ServerConfig) ValidateConfig() error {

	if c.UseTLS {
		if c.KeyPath == "" {
			return fmt.Errorf("must provide -key_file under TLS")
		}
		if !fileExists(c.KeyPath) {
			return fmt.Errorf("-key_path '%s' does not exist", c.KeyPath)
		}

		if c.CertPath == "" {
			return fmt.Errorf("must provide -key_file under TLS")
		}
		if !fileExists(c.CertPath) {
			return fmt.Errorf("-cert_path '%s' does not exist", c.CertPath)
		}
	}

	if !c.UseTLS {
		lsn, err := net.Listen("tcp", fmt.Sprintf(":%v", c.InternalLsnPort))
		if err != nil {
			return fmt.Errorf("internal port %v already bound", c.InternalLsnPort)
		}
		lsn.Close()
	}

	lsnX, err := net.Listen("tcp", fmt.Sprintf(":%v", c.ExternalLsnPort))
	if err != nil {
		return fmt.Errorf("external port %v already bound", c.ExternalLsnPort)
	}
	lsnX.Close()

	return nil
}
