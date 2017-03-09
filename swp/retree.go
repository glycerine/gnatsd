package swp

import (
	"fmt"
	"time"

	"github.com/glycerine/rbtree"
)

// retree is the retry-tree.

//msgp:ignore retree

// retree is a tree that holds TxqSlots ordered by
// their retry deadlines.
type retree struct {
	tree    *rbtree.Tree
	compare func(a, b *TxqSlot) int
}

// track one packet

func (t *retree) insert(j *TxqSlot) {
	t.tree.Insert(j)
}

func (t *retree) deleteSlot(j *TxqSlot) {
	t.tree.DeleteWithKey(j)
}

func newRetree(compareFunc func(a, b *TxqSlot) int) *retree {

	return &retree{
		tree: rbtree.NewTree(
			func(a1, b2 rbtree.Item) int {
				a := a1.(*TxqSlot)
				b := b2.(*TxqSlot)
				return compareFunc(a, b)
			}),
		compare: compareFunc,
	}
}

func compareRetryDeadline(a, b *TxqSlot) (cmp int) {
	at, bt := a.RetryDeadline.UnixNano(), b.RetryDeadline.UnixNano()
	if at != bt {
		return int(at - bt)
	}
	// keep all packets, even with tied deadlines.
	return int(a.Pack.SeqNum - b.Pack.SeqNum)
}

func compareSeqNum(a, b *TxqSlot) int {
	return int(a.Pack.SeqNum - b.Pack.SeqNum)
}

func (t *retree) deleteThroughDeadline(x time.Time, callme func(goner *TxqSlot)) {
	for it := t.tree.Min(); !it.Limit(); {
		cur := it.Item().(*TxqSlot)
		if !cur.RetryDeadline.After(x) {
			//p("retree.deleteThroughDeadline deletes \n%s\n", cur)

			next := it.Next()
			t.tree.DeleteWithIterator(it)
			if callme != nil {
				callme(cur)
			}
			it = next
		} else {
			//p("retree.deleteThroughDealine delete pass ignores \n%s\n", cur)
			break // we can stop scanning now.
		}
	}
}

func (t *retree) deleteThroughSeqNum(seqnum int64, callme func(goner *TxqSlot)) {
	for it := t.tree.Min(); !it.Limit(); {
		cur := it.Item().(*TxqSlot)
		if cur.Pack.SeqNum <= seqnum {
			//p("retree.deleteThroughSeqNum deletes \n%s\n", cur)

			next := it.Next()
			t.tree.DeleteWithIterator(it)
			if callme != nil {
				callme(cur)
			}
			it = next
		} else {
			//p("retree.deleteThroughSeqNum delete pass ignores \n%s\n", cur)
			break // we can stop scanning now.
		}
	}
}

func (t *retree) String() string {
	s := ""
	for it := t.tree.Min(); !it.Limit(); it = it.Next() {
		cur := it.Item().(*TxqSlot)
		s += fmt.Sprintf("%s\n", cur)
	}
	return s
}
