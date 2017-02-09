package health

import (
	"testing"
)

func TestBchan(t *testing.T) {

	sz := 3
	bc := NewBchan(sz)

	select {
	case <-bc.Ch:
		t.Fatal("bc starts open; it should have blocked")
	default:
		// ok, good.
	}

	aligator := "bill"
	bc.Bcast(aligator)

	for j := 0; j < 3; j++ {
		for i := 0; i < sz; i++ {
			select {
			case b := <-bc.Ch:
				bc.BcastAck()
				if b != aligator {
					t.Fatal("Bcast(aligator) means aligator should always be read on the bc")
				}
			default:
				t.Fatal("bc is now closed, should have read back aligator. Refresh should have restocked us.")
			}
		}
	}

	// multiple Set are fine:
	crocadile := "lyle"
	bc.Bcast(crocadile)

	for j := 0; j < 3; j++ {
		for i := 0; i < sz; i++ {
			select {
			case b := <-bc.Ch:
				bc.BcastAck()
				if b != crocadile {
					t.Fatal("Bcast(crocadile) means crocadile should always be read on the bc")
				}
			default:
				t.Fatal("bc is now closed, should have read back crocadile. Refresh should have restocked us.")
			}
		}
	}

	// and after Off, we should block
	bc.Clear()
	select {
	case <-bc.Ch:
		t.Fatal("Clear() means recevie should have blocked.")
	default:
		// ok, good.
	}

}
