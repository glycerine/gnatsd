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

	p("r = %s", r)
	sz := r.size()
	if sz != 2 {
		t.Fatalf("expected 2, saw sz=%v", sz)
	}
	s := r.String()
	if s == "[]" {
		t.Fatalf("missing serialization of set elements")
	}
}
