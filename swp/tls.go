package swp

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/glycerine/nats"
	"io/ioutil"
	"strings"
)

type CertConfig struct {
	initDone  bool
	certPath  string
	keyPath   string
	caPath    string
	cert      tls.Certificate
	tlsConfig *tls.Config
	rootCA    nats.Option
	skipTLS   bool
}

func (cc *CertConfig) Init(tlsDir string) {
	if tlsDir == "" {
		tlsDir = "./testfiles/"
	} else {
		tlsDir = myChompSlash(tlsDir) + "/"
	}

	cc.initDone = true
	cc.certPath = tlsDir + "certs/internal.crt"
	cc.keyPath = tlsDir + "private/internal.key"
	cc.caPath = tlsDir + "certs/ca.pem"
	//cc.skipTLS = true
	cc.skipTLS = false
}

func (cc *CertConfig) CertLoad() error {
	if !fileExists(cc.certPath) {
		return fmt.Errorf("certLoad: path '%s' does not exist", cc.certPath)
	}
	if !fileExists(cc.keyPath) {
		return fmt.Errorf("certLoad: path '%s' does not exist", cc.keyPath)
	}
	if !fileExists(cc.caPath) {
		return fmt.Errorf("certLoad: path '%s' does not exist", cc.caPath)
	}
	cert, err := tls.LoadX509KeyPair(cc.certPath, cc.keyPath)
	if err != nil {
		return fmt.Errorf("certLoad: error parsing X509 cert='%s'/key='%s', error was: '%v'",
			cc.certPath, cc.keyPath, err)
	}
	cc.cert = cert

	// nats.RootCA will do repeat this, but we detect failure earlier
	// this way and don't bother proceeding down the whole state sequence.
	pool := x509.NewCertPool()
	rootPEM, err := ioutil.ReadFile(cc.caPath)
	if err != nil || rootPEM == nil {
		err = fmt.Errorf("certLoad: error loading "+
			"rootCA file '%s': %v", cc.caPath, err)
		return err
	}
	ok := pool.AppendCertsFromPEM([]byte(rootPEM))
	if !ok {
		return fmt.Errorf("certLoad: failed to parse root certificate from %q", cc.caPath)
	}

	cc.rootCA = nats.RootCAs(cc.caPath)

	cc.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cc.cert},
		MinVersion:   tls.VersionTLS12,
	}
	return nil
}

func myChompSlash(path string) string {
	if strings.HasSuffix(path, "/") {
		return path[:len(path)-1]
	}
	return path
}
