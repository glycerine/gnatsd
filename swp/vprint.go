package swp

import (
	"fmt"
	"sync"
)

var printLock sync.Mutex

// p is a shortcut for a call to fmt.Printf that implicitly starts
// and ends its message with a newline.
func p(format string, stuff ...interface{}) {
	printLock.Lock()
	fmt.Printf("\n "+format+"\n", stuff...)
	printLock.Unlock()
}
