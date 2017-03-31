package peer

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/glycerine/cryrand"
	tun "github.com/glycerine/sshego"
)

type sshUser struct {
	mylogin  string
	myemail  string
	pw       string
	fullname string
	issuer   string
	rsaPath  string
}

type sshUserCreds struct {
	mylogin  string
	toptPath string
	qrPath   string
	rsaPath  string
}

func newTestSshUser() *sshUser {
	r := &sshUser{
		mylogin:  "bob",
		myemail:  "bob@example.com",
		fullname: "Bob Fakey McFakester",
		pw:       fmt.Sprintf("%x", string(cryrand.CryptoRandBytes(30))),
		issuer:   "hnatsd/peer/util.go",
	}
	return r
}

func (u *sshUser) addUserToSshd(srvCfg *tun.SshegoConfig) (*sshUserCreds, error) {
	// create a new acct
	toptPath, qrPath, rsaPath, err := srvCfg.HostDb.AddUser(
		u.mylogin, u.myemail, u.pw, u.issuer, u.fullname, u.rsaPath)
	if err != nil {
		return nil, err
	}
	r := &sshUserCreds{
		mylogin:  u.mylogin,
		toptPath: toptPath,
		qrPath:   qrPath,
		rsaPath:  rsaPath,
	}
	u.rsaPath = rsaPath
	return r, nil
}

// test setup utilities, exported for other packages use too.

func UtilTestSetupThree(callmePreStart func(peer *Peer)) (p0, p1, p2 *Peer, peers *LeadAndFollowList) {
	UtilCleanupTestUserDatabases()

	u := newTestSshUser()

	nPeerPort0, lsn0 := getAvailPort()
	nPeerPort1, lsn1 := getAvailPort()
	nPeerPort2, lsn2 := getAvailPort()
	nPeerPort3, lsn3 := getAvailPort()
	nPeerPort4, lsn4 := getAvailPort()
	nPeerPort5, lsn5 := getAvailPort()

	// don't close until now. Now we have non-overlapping ports.
	lsn0.Close()
	lsn1.Close()
	lsn2.Close()
	lsn3.Close()
	lsn4.Close()
	lsn5.Close()

	cluster0 := fmt.Sprintf("-cluster=nats://localhost:%v", nPeerPort2)
	cluster1 := fmt.Sprintf("-cluster=nats://localhost:%v", nPeerPort3)
	cluster2 := fmt.Sprintf("-cluster=nats://localhost:%v", nPeerPort4)
	routes1 := fmt.Sprintf("-routes=nats://localhost:%v", nPeerPort2)

	// want peer0 to be lead, so we give it lower rank.
	peer0cfg := strings.Join([]string{"-rank=0", "-health", "-p", fmt.Sprintf("%v", nPeerPort0), cluster0}, " ")

	peer1cfg := strings.Join([]string{"-rank=3", "-health", "-p", fmt.Sprintf("%v", nPeerPort1), cluster1, routes1}, " ")

	peer2cfg := strings.Join([]string{"-rank=6", "-health", "-p", fmt.Sprintf("%v", nPeerPort5), cluster2, routes1}, " ")
	var err error
	p0, err = NewPeer(peer0cfg, "p0")
	panicOn(err)
	p1, err = NewPeer(peer1cfg, "p1")
	panicOn(err)
	p2, err = NewPeer(peer2cfg, "p2")
	panicOn(err)

	p0.SshClientAllowsNewSshdServer = true
	p1.SshClientAllowsNewSshdServer = true
	p2.SshClientAllowsNewSshdServer = true

	p0.TestAllowOneshotConnect = true
	p1.TestAllowOneshotConnect = true
	p2.TestAllowOneshotConnect = true

	if callmePreStart != nil {
		callmePreStart(p0)
		callmePreStart(p1)
		callmePreStart(p2)
	}

	// start em up
	err = p0.Start()
	panicOn(err)
	err = p1.Start()
	panicOn(err)
	err = p2.Start()
	panicOn(err)

	if !p0.SkipEncryption {
		// add account to all peers, once their sshd is ready
		<-p0.SshdReady
		// 1st time sets u.rsaPath, whill will be re-used here-after.
		creds0, err := u.addUserToSshd(p0.GservCfg.SshegoCfg)
		_ = creds0
		panicOn(err)
		//p("creds0=%#v", creds0)

		<-p1.SshdReady
		creds1, err := u.addUserToSshd(p1.GservCfg.SshegoCfg)
		_ = creds1
		panicOn(err)
		//p("creds1=%#v", creds1)

		<-p2.SshdReady
		creds2, err := u.addUserToSshd(p2.GservCfg.SshegoCfg)
		_ = creds2

		panicOn(err)
		//p("creds2=%#v", creds2)

		p0.SshClientLoginUsername = u.mylogin
		p0.SshClientPrivateKeyPath = u.rsaPath
		p0.SshClientClientKnownHostsPath = "p0.sshcli.known.hosts"

		p1.SshClientLoginUsername = u.mylogin
		p1.SshClientPrivateKeyPath = u.rsaPath
		p1.SshClientClientKnownHostsPath = "p1.sshcli.known.hosts"

		p2.SshClientLoginUsername = u.mylogin
		p2.SshClientPrivateKeyPath = u.rsaPath
		p2.SshClientClientKnownHostsPath = "p2.sshcli.known.hosts"
	}

	// let peers come up and start talking
	peers, err = p0.WaitForPeerCount(3, 120*time.Second)
	if err != nil || peers == nil || len(peers.Members) != 3 {
		p("peers = %#v", peers)
		panic(fmt.Sprintf("could not setup all 3 peers?!?: err = '%v'. ", err))
	}

	return p0, p1, p2, peers
}

func UtilCleanupTestUserDatabases() {
	os.RemoveAll(".p0")
	os.RemoveAll(".p0.hostkey")
	os.RemoveAll(".p0.hostkey.pub")
	os.RemoveAll(".p1")
	os.RemoveAll(".p1.hostkey")
	os.RemoveAll(".p1.hostkey.pub")
	os.RemoveAll(".p2")
	os.RemoveAll(".p2.hostkey")
	os.RemoveAll(".p2.hostkey.pub")
	os.Remove("p0.boltdb")
	os.Remove("p1.boltdb")
	os.Remove("p2.boltdb")
	os.Remove("p0.sshcli.known.hosts")
	os.Remove("p1.sshcli.known.hosts")
	os.Remove("p2.sshcli.known.hosts")
}
