package health

import (
	"fmt"
	"reflect"
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

type members struct {
	GroupName string    `json:"GroupName"`
	Amap      *ranktree `json:"Mem"`
}

func (m *members) clear() {
	m.Amap = newRanktree()
}

func (m *members) clone() *members {
	cp := newMembers()
	cp.GroupName = m.GroupName
	if m.Amap == nil {
		return cp
	}
	cp.Amap = m.Amap.clone()
	return cp
}

func (m *members) setEmpty() bool {
	return m.Amap.Len() == 0
}

func (m *members) String() string {
	return string(m.mustJsonBytes())
}

func newMembers() *members {
	return &members{
		Amap: newRanktree(),
	}
}

func (m *members) mustJsonBytes() []byte {
	by, err := m.Amap.MarshalJSON()
	panicOn(err)
	return by
}

func slocEqual(a, b *ServerLoc) bool {
	return reflect.DeepEqual(a, b)
}
