package peer

import (
	"bytes"
	"fmt"
	"github.com/glycerine/hnatsd/peer/api"
	"github.com/glycerine/rbtree"
)

func compare(a, b *api.KeyInv) int {
	cmp := bytes.Compare(a.Key, b.Key)
	if 0 != cmp {
		return cmp
	}
	if a.When != b.When {
		if a.When.Before(b.When) {
			return -1
		}
		return 1
	}
	if a.Who == b.Who {
		return 0
	}
	if a.Who < b.Who {
		return -1
	}
	return 1
}

type Inventory struct {
	*rbtree.Tree
}

func (t *Inventory) Upsert(j *api.KeyInv) {
	t.Insert(j)
}

func (t *Inventory) deleteKi(j *api.KeyInv) {
	t.DeleteWithKey(j)
}

func NewInventory() *Inventory {

	return &Inventory{rbtree.NewTree(
		func(a1, b2 rbtree.Item) int {
			a := a1.(*api.KeyInv)
			b := b2.(*api.KeyInv)
			return compare(a, b)
		})}
}

func (t *Inventory) String() string {
	s := ""
	for it := t.Min(); !it.Limit(); it = it.Next() {
		cur := it.Item().(*api.KeyInv)
		s += fmt.Sprintf("%s\n", cur)
	}
	return s
}
