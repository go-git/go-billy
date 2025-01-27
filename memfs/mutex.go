package memfs

import "sync"

func newMutex() mutex {
	return &sync.RWMutex{}
}

func newNoOpMutex() mutex {
	return &sync.RWMutex{}
}

type mutex interface {
	Lock()
	Unlock()
	RLock()
	RUnlock()
}

type noOpMutex struct { //nolint:unused
}

func (noOpMutex) Lock() { //nolint:unused
}

func (noOpMutex) Unlock() { //nolint:unused
}

func (noOpMutex) RLock() { //nolint:unused
}

func (noOpMutex) RUnlock() { //nolint:unused
}
