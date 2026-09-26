package workbench

import "sync"

type runEventWaiters struct {
	mu   sync.Mutex
	subs map[RunID]map[chan struct{}]struct{}
}

var defaultRunEventWaiters = &runEventWaiters{subs: map[RunID]map[chan struct{}]struct{}{}}

func SubscribeRun(runID RunID) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	defaultRunEventWaiters.mu.Lock()
	waiters := defaultRunEventWaiters.subs[runID]
	if waiters == nil {
		waiters = map[chan struct{}]struct{}{}
		defaultRunEventWaiters.subs[runID] = waiters
	}
	waiters[ch] = struct{}{}
	defaultRunEventWaiters.mu.Unlock()
	return ch, func() {
		defaultRunEventWaiters.mu.Lock()
		if current := defaultRunEventWaiters.subs[runID]; current != nil {
			delete(current, ch)
			if len(current) == 0 {
				delete(defaultRunEventWaiters.subs, runID)
			}
		}
		defaultRunEventWaiters.mu.Unlock()
	}
}

func NotifyRun(runID RunID) {
	defaultRunEventWaiters.mu.Lock()
	waiters := make([]chan struct{}, 0, len(defaultRunEventWaiters.subs[runID]))
	for ch := range defaultRunEventWaiters.subs[runID] {
		waiters = append(waiters, ch)
	}
	defaultRunEventWaiters.mu.Unlock()
	for _, ch := range waiters {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
