package health

import (
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
	mu sync.RWMutex
}

func (a *ServerLoc) Less(than btree.Item) bool {
	b := than.(*ServerLoc)
	return ServerLocLessThan(a, b)
}

// insert is idemopotent so it is safe
// to insert the same sloc multiple times and
// duplicates will be ignored.
func (t *ranktree) insert(j *ServerLoc) {
	t.mu.Lock()
	t.ReplaceOrInsert(j)
	t.mu.Unlock()
}

func (t *ranktree) min() *ServerLoc {
	t.mu.RLock()
	min := t.Min()
	t.mu.RUnlock()

	if min == nil {
		return nil
	}
	return min.(*ServerLoc)
}

func (t *ranktree) deleteSloc(j *ServerLoc) {
	t.mu.Lock()
	t.Delete(j)
	t.mu.Unlock()
}

func newRanktree() *ranktree {
	return &ranktree{
		BTree: btree.New(2),
	}
}

func (t *ranktree) String() string {
	t.mu.Lock()

	s := "{"
	t.AscendGreaterOrEqual(&ServerLoc{}, func(item btree.Item) bool {
		cur := item.(*ServerLoc)
		s += fmt.Sprintf("%s,", cur)
		return true
	})
	t.mu.Unlock()

	// replace last comma with closing curly brace
	n := len(s)
	if n > 0 {
		s = s[:n-1] + "}"
	}
	return s
}

func (t *ranktree) clone() *ranktree {
	r := newRanktree()
	t.mu.Lock()

	t.AscendGreaterOrEqual(&ServerLoc{}, func(item btree.Item) bool {
		cur := item.(*ServerLoc)
		r.insert(cur)
		return true
	})
	t.mu.Unlock()
	return r
}
