package ai

import "sync"

type usageWrites struct {
	mu   sync.Mutex
	last map[string]chan struct{}
}

func writesKey(agencyID, accountID string) string { return agencyID + "|" + accountID }

func (u *usageWrites) after(agencyID, accountID string, write func()) {
	key := writesKey(agencyID, accountID)
	done := make(chan struct{})
	u.mu.Lock()
	if u.last == nil {
		u.last = map[string]chan struct{}{}
	}
	previous := u.last[key]
	u.last[key] = done
	u.mu.Unlock()
	go func() {
		if previous != nil {
			<-previous
		}
		write()
		u.mu.Lock()
		if u.last[key] == done {
			delete(u.last, key)
		}
		u.mu.Unlock()
		close(done)
	}()
}

func (u *usageWrites) settled(agencyID, accountID string) {
	u.mu.Lock()
	pending := u.last[writesKey(agencyID, accountID)]
	u.mu.Unlock()
	if pending != nil {
		<-pending
	}
}
