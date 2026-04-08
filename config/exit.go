package config

import (
	"fmt"
	"sync"
)

var (
	onExitLock sync.Mutex
	onExit     []func()
)

// OnExit registers a cleanup function to run when DoExit is called. Functions
// are executed in reverse order (LIFO), like defer statements. Panics if fn is nil.
func OnExit(fn func()) {
	if fn == nil {
		// nil value here is likely a programming error
		panic(fmt.Errorf("nil cleanup function specified"))
	}
	onExitLock.Lock()
	defer onExitLock.Unlock()
	onExit = append(onExit, fn)
}

// DoExit executes all registered exit handlers in reverse registration order.
// This is automatically called by Makefile() via defer. Thread-safe.
func DoExit() {
	onExitLock.Lock()
	defer onExitLock.Unlock()
	// reverse order, like defer
	for i := len(onExit) - 1; i >= 0; i-- {
		onExit[i]()
	}
}
