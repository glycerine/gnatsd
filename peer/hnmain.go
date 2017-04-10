package peer

import (
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/glycerine/hnatsd/auth"
	"github.com/glycerine/hnatsd/health"
	"github.com/glycerine/hnatsd/logger"
	"github.com/glycerine/hnatsd/server"
)

// We require a gnatsd instance that can parse
// its command line options. adapt the
// main.go file from gnatsd/ directly.

// Copyright 2012-2016 Apcera Inc. All rights reserved.
/*
The MIT License (MIT)

Copyright (c) 2012-2016 Apcera Inc.

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

var usageStr = `
Usage: gnatsd [options]

Server Options:
    -a, --addr <host>                Bind to host address (default: 0.0.0.0)
    -p, --port <port>                Use port for clients (default: 4222)
    -P, --pid <file>                 File to store PID
    -m, --http_port <port>           Use port for http monitoring
    -ms,--https_port <port>          Use port for https monitoring
    -c, --config <file>              Configuration file

Logging Options:
    -l, --log <file>                 File to redirect log output
    -T, --logtime                    Timestamp log entries (default: true)
    -s, --syslog                     Log to syslog or windows event log
    -r, --remote_syslog <addr>       Syslog server addr (udp://localhost:514)
    -D, --debug                      Enable debugging output
    -V, --trace                      Trace the raw protocol
    -DV                              Debug and trace

Authorization Options:
        --user <user>                User required for connections
        --pass <password>            Password required for connections
        --auth <token>               Authorization token required for connections

TLS Options:
        --tls                        Enable TLS, do not verify clients (default: false)
        --tlscert <file>             Server certificate file
        --tlskey <file>              Private key for server certificate
        --tlsverify                  Enable TLS, verify client certificates
        --tlscacert <file>           Client certificate CA for verification

Cluster Options:
        --routes <rurl-1, rurl-2>    Routes to solicit and connect
        --cluster <cluster-url>      Cluster URL for solicited routes
        --no_advertise <bool>        Advertise known cluster IPs to clients
        --connect_retries <number>   For implicit routes, number of connect retries

Cluster Health Monitor:
        --health                     Run the health monitoring/leader election agent
        --lease <duration>           Duration of leader leases. default: 12s
        --beat  <duration>           Time between heartbeats (want 3-4/lease). default: 3s
        --rank  <number>             Smaller rank gives priority in leader election

Common Options:
    -h, --help                       Show this message
    -v, --version                    Show version
        --help_tls                   TLS help
`

// usage will print out the flag options for the server.
func usage() {
	fmt.Printf("%s\n", usageStr)
	os.Exit(0)
}

func hnatsdMain(args []string) (*server.Server, *server.Options, error) {
	// Server Options
	opts := server.Options{}

	fs := flag.NewFlagSet("hnatsd_peer", flag.ExitOnError)

	var showVersion bool
	var debugAndTrace bool
	var configFile string
	var showTLSHelp bool

	// Parse flags
	fs.IntVar(&opts.Port, "port", 0, "Port to listen on.")
	fs.IntVar(&opts.Port, "p", 0, "Port to listen on.")
	fs.StringVar(&opts.Host, "addr", "", "Network host to listen on.")
	fs.StringVar(&opts.Host, "a", "", "Network host to listen on.")
	fs.StringVar(&opts.Host, "net", "", "Network host to listen on.")
	fs.BoolVar(&opts.Debug, "D", false, "Enable Debug logging.")
	fs.BoolVar(&opts.Debug, "debug", false, "Enable Debug logging.")
	fs.BoolVar(&opts.Trace, "V", false, "Enable Trace logging.")
	fs.BoolVar(&opts.Trace, "trace", false, "Enable Trace logging.")
	fs.BoolVar(&debugAndTrace, "DV", false, "Enable Debug and Trace logging.")
	fs.BoolVar(&opts.Logtime, "T", true, "Timestamp log entries.")
	fs.BoolVar(&opts.Logtime, "logtime", true, "Timestamp log entries.")
	fs.StringVar(&opts.Username, "user", "", "Username required for connection.")
	fs.StringVar(&opts.Password, "pass", "", "Password required for connection.")
	fs.StringVar(&opts.Authorization, "auth", "", "Authorization token required for connection.")
	fs.IntVar(&opts.HTTPPort, "m", 0, "HTTP Port for /varz, /connz endpoints.")
	fs.IntVar(&opts.HTTPPort, "http_port", 0, "HTTP Port for /varz, /connz endpoints.")
	fs.IntVar(&opts.HTTPSPort, "ms", 0, "HTTPS Port for /varz, /connz endpoints.")
	fs.IntVar(&opts.HTTPSPort, "https_port", 0, "HTTPS Port for /varz, /connz endpoints.")
	fs.StringVar(&configFile, "c", "", "Configuration file.")
	fs.StringVar(&configFile, "config", "", "Configuration file.")
	fs.StringVar(&opts.PidFile, "P", "", "File to store process pid.")
	fs.StringVar(&opts.PidFile, "pid", "", "File to store process pid.")
	fs.StringVar(&opts.LogFile, "l", "", "File to store logging output.")
	fs.StringVar(&opts.LogFile, "log", "", "File to store logging output.")
	fs.BoolVar(&opts.Syslog, "s", false, "Enable syslog as log method.")
	fs.BoolVar(&opts.Syslog, "syslog", false, "Enable syslog as log method..")
	fs.StringVar(&opts.RemoteSyslog, "r", "", "Syslog server addr (udp://localhost:514).")
	fs.StringVar(&opts.RemoteSyslog, "remote_syslog", "", "Syslog server addr (udp://localhost:514).")
	fs.BoolVar(&showVersion, "version", false, "Print version information.")
	fs.BoolVar(&showVersion, "v", false, "Print version information.")
	fs.IntVar(&opts.ProfPort, "profile", 0, "Profiling HTTP port")
	fs.StringVar(&opts.RoutesStr, "routes", "", "Routes to actively solicit a connection.")
	fs.StringVar(&opts.Cluster.ListenStr, "cluster", "", "Cluster url from which members can solicit routes.")
	fs.StringVar(&opts.Cluster.ListenStr, "cluster_listen", "", "Cluster url from which members can solicit routes.")
	fs.BoolVar(&opts.Cluster.NoAdvertise, "no_advertise", false, "Advertise known cluster IPs to clients.")
	fs.IntVar(&opts.Cluster.ConnectRetries, "connect_retries", 0, "For implicit routes, number of connect retries")
	fs.BoolVar(&showTLSHelp, "help_tls", false, "TLS help.")
	fs.BoolVar(&opts.TLS, "tls", false, "Enable TLS.")
	fs.BoolVar(&opts.TLSVerify, "tlsverify", false, "Enable TLS with client verification.")
	fs.StringVar(&opts.TLSCert, "tlscert", "", "Server certificate file.")
	fs.StringVar(&opts.TLSKey, "tlskey", "", "Private key for server certificate.")
	fs.StringVar(&opts.TLSCaCert, "tlscacert", "", "Client certificate CA for verification.")
	fs.BoolVar(&opts.HealthAgent, "health", false, "Run the health agent, elect a leader.")
	fs.IntVar(&opts.HealthRank, "rank", 7, "leader election priority: the smaller the rank, the more preferred the server is as a leader. Negative ranks are allowed. Ties are broken by the random ServerId.")
	fs.DurationVar(&opts.HealthLease, "lease", time.Second*12, "leader lease duration (should allow 3-4 beats within a lease)")
	fs.DurationVar(&opts.HealthBeat, "beat", time.Second*3, "heart beat every this often (should get 3-4 beats within a lease)")

	fs.Usage = func() {
		fmt.Printf("%s\n", usageStr)
	}

	fs.Parse(args)

	// Show version and exit
	if showVersion {
		server.PrintServerAndExit()
	}

	if showTLSHelp {
		server.PrintTLSHelpAndDie()
	}

	// One flag can set multiple options.
	if debugAndTrace {
		opts.Trace, opts.Debug = true, true
	}

	// Process args looking for non-flag options,
	// 'version' and 'help' only for now
	showVersion, showHelp, err := server.ProcessCommandLineArgs(fs)
	if err != nil {
		//p("peer.go: err in server.ProcessCommandLineArgs err='%v' from fs='%#v'", err, fs)
		return nil, nil, err
	} else if showVersion {
		server.PrintServerAndExit()
	} else if showHelp {
		usage()
	}

	// Parse config if given
	if configFile != "" {
		fileOpts, err := server.ProcessConfigFile(configFile)
		if err != nil {
			server.PrintAndDie(err.Error())
		}
		opts = *server.MergeOptions(fileOpts, &opts)
	}

	// Remove any host/ip that points to itself in Route
	newroutes, err := server.RemoveSelfReference(opts.Cluster.Port, opts.Routes)
	if err != nil {
		server.PrintAndDie(err.Error())
	}
	opts.Routes = newroutes

	// Configure TLS based on any present flags
	configureTLS(&opts)

	// Configure cluster opts if explicitly set via flags.
	err = configureClusterOpts(&opts)
	if err != nil {
		server.PrintAndDie(err.Error())
	}

	if opts.HealthAgent {
		opts.InternalCli = append(opts.InternalCli, health.NewAgent(&opts))
	}

	// Create the server with appropriate options.
	s := server.New(&opts)

	// Configure the authentication mechanism
	configureAuth(s, &opts)

	// Configure the logger based on the flags
	configureLogger(s, &opts)

	return s, &opts, nil
}

func configureAuth(s *server.Server, opts *server.Options) {
	// Client
	// Check for multiple users first
	if opts.Users != nil {
		auth := auth.NewMultiUser(opts.Users)
		s.SetClientAuthMethod(auth)
	} else if opts.Username != "" {
		auth := &auth.Plain{
			Username: opts.Username,
			Password: opts.Password,
		}
		s.SetClientAuthMethod(auth)
	} else if opts.Authorization != "" {
		auth := &auth.Token{
			Token: opts.Authorization,
		}
		s.SetClientAuthMethod(auth)
	}
	// Routes
	if opts.Cluster.Username != "" {
		auth := &auth.Plain{
			Username: opts.Cluster.Username,
			Password: opts.Cluster.Password,
		}
		s.SetRouteAuthMethod(auth)
	}
}

func configureLogger(s *server.Server, opts *server.Options) {
	var log server.Logger

	if opts.LogFile != "" {
		log = logger.NewFileLogger(opts.LogFile, opts.Logtime, opts.Debug, opts.Trace, true, 0)
	} else if opts.RemoteSyslog != "" {
		log = logger.NewRemoteSysLogger(opts.RemoteSyslog, opts.Debug, opts.Trace)
	} else if opts.Syslog {
		log = logger.NewSysLogger(opts.Debug, opts.Trace)
	} else {
		colors := true
		// Check to see if stderr is being redirected and if so turn off color
		// Also turn off colors if we're running on Windows where os.Stderr.Stat() returns an invalid handle-error
		stat, err := os.Stderr.Stat()
		if err != nil || (stat.Mode()&os.ModeCharDevice) == 0 {
			colors = false
		}
		// hnatds: use the Go standard library's log facility
		// for ease in embedding.
		log = logger.NewStdLogLogger(opts.Logtime, opts.Debug, opts.Trace, colors, true, 0)

		// original gnatsd invoked:
		//log = logger.NewStdLogger(opts.Logtime, opts.Debug, opts.Trace, colors, true, 0)
	}

	s.SetLogger(log, opts.Debug, opts.Trace)
}

func configureTLS(opts *server.Options) {
	// If no trigger flags, ignore the others
	if !opts.TLS && !opts.TLSVerify {
		return
	}
	if opts.TLSCert == "" {
		server.PrintAndDie("TLS Server certificate must be present and valid.")
	}
	if opts.TLSKey == "" {
		server.PrintAndDie("TLS Server private key must be present and valid.")
	}

	tc := server.TLSConfigOpts{}
	tc.CertFile = opts.TLSCert
	tc.KeyFile = opts.TLSKey
	tc.CaFile = opts.TLSCaCert

	if opts.TLSVerify {
		tc.Verify = true
	}
	var err error
	if opts.TLSConfig, err = server.GenTLSConfig(&tc); err != nil {
		server.PrintAndDie(err.Error())
	}
}

func configureClusterOpts(opts *server.Options) error {
	// If we don't have cluster defined in the configuration
	// file and no cluster listen string override, but we do
	// have a routes override, we need to report misconfiguration.
	if opts.Cluster.ListenStr == "" && opts.Cluster.Host == "" &&
		opts.Cluster.Port == 0 {
		if opts.RoutesStr != "" {
			server.PrintAndDie("Solicited routes require cluster capabilities, e.g. --cluster.")
		}
		return nil
	}

	// If cluster flag override, process it
	if opts.Cluster.ListenStr != "" {
		clusterURL, err := url.Parse(opts.Cluster.ListenStr)
		if err != nil {
			return err
		}
		h, p, err := net.SplitHostPort(clusterURL.Host)
		if err != nil {
			return err
		}
		opts.Cluster.Host = h
		_, err = fmt.Sscan(p, &opts.Cluster.Port)
		if err != nil {
			return err
		}

		if clusterURL.User != nil {
			pass, hasPassword := clusterURL.User.Password()
			if !hasPassword {
				return fmt.Errorf("Expected cluster password to be set.")
			}
			opts.Cluster.Password = pass

			user := clusterURL.User.Username()
			opts.Cluster.Username = user
		} else {
			// Since we override from flag and there is no user/pwd, make
			// sure we clear what we may have gotten from config file.
			opts.Cluster.Username = ""
			opts.Cluster.Password = ""
		}
	}

	// If we have routes but no config file, fill in here.
	if opts.RoutesStr != "" && opts.Routes == nil {
		opts.Routes = server.RoutesFromStr(opts.RoutesStr)
	}

	return nil
}
