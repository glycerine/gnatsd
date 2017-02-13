package health

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

// utilities and sets stuff

// p is a shortcut for a call to fmt.Printf that implicitly starts
// and ends its message with a newline.
func p(format string, stuff ...interface{}) {
	//fmt.Printf("\n "+format+"\n", stuff...)
}

func panicOn(err error) {
	if err != nil {
		panic(err)
	}
}

// return a minus b, where a and b are sets.
func setDiff(a, b *members, curLead *ServerLoc) *members {

	res := newMembers()
	a.Amap.tex.Lock()
	for k, v := range a.Amap.U {

		if curLead != nil {
			// annotate leader as we go...
			if v.Id == curLead.Id {
				v.IsLeader = true
				v.LeaseExpires = curLead.LeaseExpires
			}
		}

		if _, found := b.Amap.U[k]; !found { // data race
			res.Amap.U[k] = v
		}
	}
	a.Amap.tex.Unlock()
	return res
}

func setsEqual(a, b *members) bool {
	a.Amap.tex.Lock()
	b.Amap.tex.Lock()
	defer b.Amap.tex.Unlock()
	defer a.Amap.tex.Unlock()

	alen := len(a.Amap.U)
	if alen != len(b.Amap.U) {
		return false
	}
	// INVAR: len(a) == len(b)
	if alen == 0 {
		return true
	}
	for k := range a.Amap.U {
		if _, found := b.Amap.U[k]; !found {
			return false
		}
	}
	// INVAR: all of a was found in b, and they
	// are the same size
	return true
}

type members struct {
	GroupName string              `json:"GroupName"`
	Amap      *AtomicServerLocMap `json:"Mem"`
}

func (m *members) clear() {
	m.Amap = NewAtomicServerLocMap()
}

func (m *members) clone() *members {
	cp := newMembers()
	cp.GroupName = m.GroupName
	if m.Amap == nil {
		return cp
	}
	m.Amap.tex.Lock()
	cp.Amap.tex.Lock()
	for k, v := range m.Amap.U {
		cp.Amap.U[k] = v
	}
	cp.Amap.tex.Unlock()
	m.Amap.tex.Unlock()
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
		Amap: NewAtomicServerLocMap(),
	}
}

func (m members) mustJsonBytes() []byte {
	m.Amap.tex.Lock()
	defer m.Amap.tex.Unlock()

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "[")
	i := 0
	n := len(m.Amap.U)
	for _, v := range m.Amap.U {
		by, err := json.Marshal(v)
		panicOn(err)
		buf.Write(by)
		if i < n-1 {
			fmt.Fprintf(&buf, ",")
		}
		i++
	}
	fmt.Fprintf(&buf, "]")

	return buf.Bytes()
}

func slocEqual(a, b *ServerLoc) bool {
	return reflect.DeepEqual(a, b)
}
