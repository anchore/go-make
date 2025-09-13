package config

import (
	"fmt"
	"sync"
)

var (
	onExitLock sync.Mutex
	onExit     []func()
)

func OnExit(fn func()) {
	if fn == nil {
		// nil value here is likely a programming error
		panic(fmt.Errorf("nil cleanup function specified"))
	}
	onExitLock.Lock()
	defer onExitLock.Unlock()
	onExit = append(onExit, fn)
}

func DoExit() {
	onExitLock.Lock()
	defer onExitLock.Unlock()
	// reverse order, like defer
	for i := len(onExit) - 1; i >= 0; i-- {
		onExit[i]()
	}
}
