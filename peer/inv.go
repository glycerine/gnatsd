package peer

import (
	"bytes"
	"fmt"

	"github.com/glycerine/rbtree"
)

func (k *KeyInv) String() string {
	return fmt.Sprintf(`{Key:"%s", Who:"%s", When:"%v"}`,
		string(k.Key), k.Who, k.When.UTC())
}

func compare(a, b *KeyInv) int {
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

func (t *Inventory) Upsert(j *KeyInv) {
	t.Insert(j)
}

func (t *Inventory) deleteKi(j *KeyInv) {
	t.DeleteWithKey(j)
}

func NewInventory() *Inventory {

	return &Inventory{rbtree.NewTree(
		func(a1, b2 rbtree.Item) int {
			a := a1.(*KeyInv)
			b := b2.(*KeyInv)
			return compare(a, b)
		})}
}

func (t *Inventory) String() string {
	s := ""
	for it := t.Min(); !it.Limit(); it = it.Next() {
		cur := it.Item().(*KeyInv)
		s += fmt.Sprintf("%s\n", cur)
	}
	return s
}
