package health

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

func Example() {

	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage()
	}

	rank := 0
	var err error
	if len(args) >= 2 {
		rank, err = strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "2nd arg should be our numeric rank")
		}
	}

	cfg := &MembershipCfg{
		MaxClockSkew: time.Second,
		BeatDur:      100 * time.Millisecond,
		NatsUrl:      "nats://" + args[0], // "nats://127.0.0.1:4222"
		MyRank:       rank,
	}
	m := NewMembership(cfg)
	panicOn(m.Start())
}
