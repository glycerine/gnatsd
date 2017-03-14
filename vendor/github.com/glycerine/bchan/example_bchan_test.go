package bchan_test

import (
	"fmt"
	"github.com/glycerine/bchan"
	"time"
)

func Example() {

	b := bchan.New(3)
	go func() {
		for {
			select {
			case v := <-b.Ch:
				b.BcastAck() // always do this after a receive on Ch, to keep broadcast enabled.
				fmt.Printf("received on Ch: %v\n", v)
			}
		}
	}()

	b.Set(4)
	time.Sleep(20 * time.Millisecond)
	b.Set(5)
	time.Sleep(20 * time.Millisecond)
	b.Clear()
	time.Sleep(20 * time.Millisecond)
}
