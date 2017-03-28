package peer

import (
	"fmt"

	"github.com/glycerine/cryrand"
	tun "github.com/glycerine/sshego"
)

type sshUser struct {
	mylogin  string
	myemail  string
	pw       string
	fullname string
	issuer   string
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
		u.mylogin, u.myemail, u.pw, u.issuer, u.fullname)
	if err != nil {
		return nil, err
	}
	r := &sshUserCreds{
		mylogin:  u.mylogin,
		toptPath: toptPath,
		qrPath:   qrPath,
		rsaPath:  rsaPath,
	}
	return r, nil
}
