package health

import (
	"testing"
)

func Test201BtreeInsertDisplay(t *testing.T) {
	s1 := &ServerLoc{Id: "abc"}
	s2 := &ServerLoc{Id: "xyz"}
	r := newRanktree()
	r.insert(s2)
	r.insert(s1)

	sz := r.size()
	if sz != 2 {
		t.Fatalf("expected 2, saw sz=%v", sz)
	}
	s := r.String()
	if s == "[]" {
		t.Fatalf("missing serialization of set elements")
	}
	expect := `[{"serverId":"abc","host":"","port":0,"leader":false,"leaseExpires":"0001-01-01T00:00:00Z","rank":0},{"serverId":"xyz","host":"","port":0,"leader":false,"leaseExpires":"0001-01-01T00:00:00Z","rank":0}]`
	if s != expect {
		t.Fatalf("serial json didn't match expectations.\n expect:'%s'\n\n observe:'%s'", expect, s)
	}
}

func Test202BtreeEqual(t *testing.T) {
	s1 := &ServerLoc{Id: "abc"}
	s2 := &ServerLoc{Id: "xyz"}
	r := newRanktree()
	r.insert(s2)
	r.insert(s1)

	s := r.clone()
	same := setsEqual(&members{DedupTree: s}, &members{DedupTree: r})
	if !same {
		t.Fatalf("expected setsEqual to be true")
	}
}
