package health

import (
	"encoding/json"
	"fmt"
)

// utilities and sets stuff

// p is a shortcut for a call to fmt.Printf that implicitly starts
// and ends its message with a newline.
func p(format string, stuff ...interface{}) {
	fmt.Printf("\n "+format+"\n", stuff...)
}

func panicOn(err error) {
	if err != nil {
		panic(err)
	}
}

// return a minus b, where a and b are sets.
func setDiff(a, b *members, curLead *ServerLoc) *members {

	res := newMembers()
	for k, v := range a.Mem {

		if curLead != nil {
			// annotate leader as we go...
			if v.Id == curLead.Id {
				v.IsLeader = true
				v.LeaseExpires = curLead.LeaseExpires
			}
		}

		if _, found := b.Mem[k]; !found {
			res.Mem[k] = v
		}
	}
	return res
}

func setsEqual(a, b *members) bool {
	if len(a.Mem) != len(b.Mem) {
		return false
	}
	// INVAR: len(a) == len(b)
	if len(a.Mem) == 0 {
		return true
	}
	for k := range a.Mem {
		if _, found := b.Mem[k]; !found {
			return false
		}
	}
	// INVAR: all of a was found in b, and they
	// are the same size
	return true
}

type members struct {
	GroupName string                `json:"GroupName"`
	Mem       map[string]*ServerLoc `json:"Mem"`
}

func (m *members) setEmpty() bool {
	return len(m.Mem) == 0
}

func (m *members) String() string {
	return string(m.mustJsonBytes())
}

func newMembers() *members {
	return &members{
		Mem: make(map[string]*ServerLoc),
	}
}

func (m members) mustJsonBytes() []byte {
	by, err := json.Marshal(m)
	panicOn(err)
	return by
}
