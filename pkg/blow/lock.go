package blow

import "sync"

type ConditionLock struct {
	lock sync.Mutex
}

func (c *ConditionLock) Lock()   { c.lock.Lock() }
func (c *ConditionLock) Unlock() { c.lock.Unlock() }

type noLock struct{}

func (noLock) Lock()     {}
func (n noLock) Unlock() {}

func NewConditionalLock(condition bool) sync.Locker {
	if condition {
		return &ConditionLock{}
	}

	return &noLock{}
}
