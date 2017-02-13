package health

import (
	"bytes"

	"fmt"
	"sync"

	"github.com/google/btree"
)

// ranktree is an in-memory, sorted,
// balanced tree that is implemented
// as a left-learning red-black tree.
// It holds *ServerLoc
// from candidate servers in the cluster,
// sorting them based on
// ServerLocLessThan() so they are in
// priority order and deduplicated.
type ranktree struct {
	*btree.BTree
	tex sync.Mutex
}

func (a *ServerLoc) Less(than btree.Item) bool {
	b := than.(*ServerLoc)
	return ServerLocLessThan(a, b)
}

// insert is idemopotent so it is safe
// to insert the same sloc multiple times and
// duplicates will be ignored.
func (t *ranktree) insert(j *ServerLoc) {
	t.tex.Lock()
	t.ReplaceOrInsert(j)
	t.tex.Unlock()
}

func (t *ranktree) present(j *ServerLoc) bool {
	t.tex.Lock()
	b := t.Has(j)
	t.tex.Unlock()
	return b
}

func (t *ranktree) minrank() *ServerLoc {
	t.tex.Lock()
	min := t.Min()
	t.tex.Unlock()

	if min == nil {
		return nil
	}
	return min.(*ServerLoc)
}

func (t *ranktree) deleteSloc(j *ServerLoc) {
	t.tex.Lock()
	t.Delete(j)
	t.tex.Unlock()
}

func newRanktree() *ranktree {
	return &ranktree{
		BTree: btree.New(2),
	}
}

func (t *ranktree) String() string {
	t.tex.Lock()

	s := "["
	t.AscendLessThan(&ServerLoc{}, func(item btree.Item) bool {
		cur := item.(*ServerLoc)
		s += cur.String()
		return true
	})
	t.tex.Unlock()

	// replace last comma with matching bracket
	n := len(s)
	if n > 1 {
		s = s[:n-1]
	}
	return s + "]"
}

func (t *ranktree) clone() *ranktree {
	r := newRanktree()
	t.tex.Lock()

	t.AscendLessThan(&ServerLoc{}, func(item btree.Item) bool {
		cur := item.(*ServerLoc)
		r.insert(cur)
		return true
	})
	t.tex.Unlock()
	return r
}

func (t *ranktree) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s", t)
	return buf.Bytes(), nil
}

// Len() is inherted from the btree,
// but isn't protected by mutex.
func (t *ranktree) size() int {
	t.tex.Lock()
	n := t.Len()
	t.tex.Unlock()
	return n
}

// return a minus b, where a and b are sets.
func setDiff(a, b *members, curLead *ServerLoc) *members {

	res := a.Amap.clone()
	a.Amap.tex.Lock()
	b.Amap.tex.Lock()

	b.Amap.AscendLessThan(&ServerLoc{}, func(item btree.Item) bool {
		v := item.(*ServerLoc)
		res.deleteSloc(v)

		if curLead != nil {
			// annotate leader as we go...
			if v.Id == curLead.Id {
				v.IsLeader = true
				v.LeaseExpires = curLead.LeaseExpires
			}
		}
		return true // keep iterating
	})

	b.Amap.tex.Unlock()
	a.Amap.tex.Unlock()
	return &members{Amap: res}
}

func setsEqual(a, b *members) bool {
	a.Amap.tex.Lock()
	b.Amap.tex.Lock()
	defer b.Amap.tex.Unlock()
	defer a.Amap.tex.Unlock()

	alen := a.Amap.Len()
	if alen != b.Amap.Len() {
		return false
	}
	// INVAR: len(a) == len(b)
	if alen == 0 {
		return true
	}

	missing := false
	a.Amap.AscendLessThan(&ServerLoc{}, func(item btree.Item) bool {
		v := item.(*ServerLoc)
		if !b.Amap.present(v) {
			missing = true
			return false // stop iterating
		}
		return true // keep iterating
	})
	return !missing
}
