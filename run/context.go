package run

import (
	"context"
	"sync"
)

var (
	contextLock            = &sync.Mutex{}
	currentContext, cancel = context.WithCancel(context.Background())
)

// Context returns the current context being used for executing scripts
func Context() context.Context {
	contextLock.Lock()
	defer contextLock.Unlock()
	return currentContext
}

// SetContext sets the context to be used for executing scripts
func SetContext(ctx context.Context) {
	contextLock.Lock()
	defer contextLock.Unlock()
	currentContext, cancel = context.WithCancel(ctx)
}

// Cancel cancels any currently executing scripts, which used the current context
func Cancel() {
	contextLock.Lock()
	defer contextLock.Unlock()
	cancel()
	currentContext, cancel = context.WithCancel(context.Background())
}
