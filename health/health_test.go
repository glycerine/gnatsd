package health

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	cv "github.com/glycerine/goconvey/convey"
	"github.com/nats-io/gnatsd/logger"
	"github.com/nats-io/gnatsd/server"
	gnatsd "github.com/nats-io/gnatsd/test"
	"github.com/nats-io/go-nats"
)

const TEST_PORT = 8392
const DefaultTimeout = 2 * time.Second

var cliOpts = nats.Options{
	Url:            fmt.Sprintf("nats://localhost:%d", TEST_PORT),
	AllowReconnect: true,
	MaxReconnect:   10,
	ReconnectWait:  10 * time.Millisecond,
	Timeout:        DefaultTimeout,
}

// DefaultTestOptions are default options for the unit tests.
var serverOpts = server.Options{
	Host:           "localhost",
	Port:           TEST_PORT,
	NoLog:          true,
	NoSigs:         true,
	MaxControlLine: 256,
}

func Test101StressTestManyClients(t *testing.T) {

	cv.Convey("when stress testing with 50 clients coming up and shutting down, we should survive and prosper", t, func() {

		s := RunServerOnPort(TEST_PORT)
		defer s.Shutdown()

		n := 50
		var ms []*Membership
		for i := 0; i < n; i++ {
			cli, srv, err := NewInternalClientPair()
			panicOn(err)

			s.InternalCliRegisterCallback(srv)
			cfg := &MembershipCfg{
				CliConn:      cli,
				MaxClockSkew: 1 * time.Nanosecond,
				LeaseTime:    30 * time.Millisecond,
				BeatDur:      10 * time.Millisecond,
				NatsUrl:      fmt.Sprintf("nats://localhost:%v", TEST_PORT),
				MyRank:       i, // ranks 0..n-1
			}

			m := NewMembership(cfg)
			err = m.Start()
			if err != nil {
				panic(err)
			}
			ms = append(ms, m)
			defer m.Stop()
		}
		// the test here is basically that we didn't crash
		// or hang. So if we got here, success.
		cv.So(true, cv.ShouldBeTrue)
	})
}

func Test102ConvergenceToOneLowRankLeaderAndLiveness(t *testing.T) {

	cv.Convey("Given a cluster of one server with rank 0, no matter what other servers arrive thinking they are the leader (say, after a partition is healed), as long as those other nodes have rank 1, our rank 0 process will persist in leading and all other arrivals will give up their leadership claims (after their leases expire). In addition to safety, this is also a liveness check: After a single lease term + clockskew, a leader will have been chosen.", t, func() {

		s := RunServerOnPort(TEST_PORT)
		defer func() {
			p("starting gnatsd shutdown...")
			s.Shutdown()
		}()

		n := 50
		tot := 50
		pause := make([]int, n)
		for i := 0; i < n; i++ {
			pause[i] = 20 + rand.Intn(50)
			tot += pause[i]
		}

		var ms []*Membership
		for i := 0; i < n; i++ {

			cfg := &MembershipCfg{
				MaxClockSkew: 1 * time.Nanosecond,
				LeaseTime:    150 * time.Millisecond,
				BeatDur:      50 * time.Millisecond,
				NatsUrl:      fmt.Sprintf("nats://localhost:%v", TEST_PORT),
				MyRank:       i,         //min(1, i), // ranks 0,1,1,1,1,1,...
				deaf:         DEAF_TRUE, // don't ping or pong
				historyCount: 10000,
			}

			cli, srv, err := NewInternalClientPair()
			panicOn(err)

			s.InternalCliRegisterCallback(srv)
			cfg.CliConn = cli

			if i == 0 {
				cfg.deaf = DEAF_FALSE
				aLogger := logger.NewStdLogger(micros, true, trace, colors, pid)
				_ = aLogger
				// to follow the prints, uncomment:
				//cfg.Log = aLogger
			}

			m := NewMembership(cfg)
			err = m.Start()
			if err != nil {
				panic(err)
			}
			ms = append(ms, m)
			defer m.Stop()
		}

		// let them all get past init phase.
		time.Sleep(2 * (ms[0].Cfg.LeaseTime + ms[0].Cfg.MaxClockSkew))

		// verify liveness, a leader exists.
		p("verifying everyone thinks there is a leader:")
		for i := 0; i < n; i++ {
			//fmt.Printf("verifying %v thinks there is a leader\n", i)
			cv.So(ms[i].elec.history.Avail(), cv.ShouldBeGreaterThan, 0)
		}

		// bring in jobs after their random pause time
		for i := 0; i < n; i++ {
			dur := time.Duration(pause[i]) * time.Millisecond
			//p("%v  on i = %v/dur=%v ", time.Now().UTC(), i, dur)
			time.Sleep(dur)
			ms[i].unDeaf()
		}

		// check that the history from rank 0
		// always shows rank 0 as lead.
		h := ms[0].elec.history
		av := h.Avail()
		//p("ms[0].myLoc.Id = %v", ms[0].myLoc.Id)
		cv.So(ms[0].myLoc.Id, cv.ShouldNotEqual, "")
		cv.So(av, cv.ShouldBeGreaterThan, 10)
		p("av: available history len = %v", av)

		// prints first:

		for i := 0; i < av; i++ {
			sloc := h.A[h.Kth(i)].(*ServerLoc)
			fmt.Printf("history print i = %v. sloc.Id=%v / sloc.Rank=%v, port=%v\n", i, sloc.Id, sloc.Rank, sloc.Port)
		}
		// checks second:
		for i := 0; i < av; i++ {
			sloc := h.A[h.Kth(i)].(*ServerLoc)
			//fmt.Printf("history check Id at i = %v. sloc.Id=%v\n", i, sloc.Id)
			cv.So(sloc.Id, cv.ShouldEqual, ms[0].myLoc.Id)
			// ports will be the only thing different when
			// running off of the one gnatsd that has the
			// same rank and Id for all clients.
			cv.So(sloc.Port, cv.ShouldEqual, ms[0].myLoc.Port)
		}

		for i := 0; i < av; i++ {
			sloc := h.A[h.Kth(i)].(*ServerLoc)
			//p("history check Rank at i = %v. sloc.Rank=%v", i, sloc.Rank)
			cv.So(sloc.Rank, cv.ShouldEqual, 0)
		}
	})
}

func Test103TiedRanksUseIdAndDoNotAlternate(t *testing.T) {

	cv.Convey("Given a cluster of two servers with rank 0 and different IDs, one should win after the initial period, and they should not alternate leadership as they carry forward.", t, func() {

		s := RunServerOnPort(TEST_PORT)
		defer func() {
			p("starting gnatsd shutdown...")
			s.Shutdown()
		}()

		n := 2

		var ms []*Membership
		for i := 0; i < n; i++ {

			cfg := &MembershipCfg{
				MaxClockSkew: 1 * time.Nanosecond,
				LeaseTime:    400 * time.Millisecond,
				BeatDur:      100 * time.Millisecond,
				NatsUrl:      fmt.Sprintf("nats://localhost:%v", TEST_PORT),
				MyRank:       0,
				historyCount: 10000,
			}

			cli, srv, err := NewInternalClientPair()
			panicOn(err)

			s.InternalCliRegisterCallback(srv)
			cfg.CliConn = cli

			aLogger := logger.NewStdLogger(micros, true, trace, colors, pid)
			cfg.Log = aLogger

			m := NewMembership(cfg)
			err = m.Start()
			if err != nil {
				panic(err)
			}
			ms = append(ms, m)
			defer m.Stop()
		}

		// let them all get past init phase.
		time.Sleep(2 * (ms[0].Cfg.LeaseTime + ms[0].Cfg.MaxClockSkew))

		// verify liveness, a leader exists.
		p("verifying everyone thinks there is a leader:")
		for i := 0; i < n; i++ {
			fmt.Printf("verifying %v thinks there is a leader\n", i)
			cv.So(ms[i].elec.history.Avail(), cv.ShouldBeGreaterThan, 0)
		}

		// sleep for 10 lease cycles - check for alternation
		time.Sleep(10 * (ms[0].Cfg.LeaseTime + ms[0].Cfg.MaxClockSkew))

		// who should be winner after lease expiration...
		zeroWins := ServerLocLessThan(&ms[0].myLoc, &ms[1].myLoc, time.Now().Add(time.Hour))
		p("zeroWins: %v, [0].myLoc=%v  [1].myLoc=%v", zeroWins, &ms[0].myLoc, &ms[1].myLoc)
		winner := &ms[1].myLoc
		if zeroWins {
			winner = &ms[0].myLoc
		}

		for j := 0; j < n; j++ {

			// check that the history doesn't alternate
			// between ports / servers.
			h := ms[j].elec.history
			av := h.Avail()
			p("ms[j=%v].myLoc.Id = %v", j, ms[j].myLoc.Id)
			p("av: j=%v, available history len = %v", j, av)

			// prints first:
			for i := 0; i < av; i++ {
				sloc := h.A[h.Kth(i)].(*ServerLoc)
				fmt.Printf("server j=%v, history print i = %v. sloc.Id=%v / sloc.Port=%v\n", j, i, sloc.Id, sloc.Port)
			}

			// checks second:
			for i := 0; i < av; i++ {
				sloc := h.A[h.Kth(i)].(*ServerLoc)
				fmt.Printf("server j=%v, history check Id at i = %v. sloc.Id=%v  and winner.Port=%v\n", j, i, sloc.Id, winner.Port)
				cv.So(sloc.Port, cv.ShouldEqual, winner.Port)
			}

		}
	})
}

func RunServerOnPort(port int) *server.Server {
	opts := serverOpts
	opts.Port = port
	return gnatsd.RunServer(&opts)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
