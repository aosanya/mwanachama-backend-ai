package ai

import (
	"context"
	"hash/fnv"
	"sync"
	"sync/atomic"
)

const holdGates = 64

type holdKey struct{ agencyID, accountID, roleID string }

type requestHolds struct {
	gates  [holdGates]sync.Mutex
	mu     sync.Mutex
	counts map[holdKey]int
}

func (h *requestHolds) gate(agencyID, accountID string) *sync.Mutex {
	sum := fnv.New32a()
	_, _ = sum.Write([]byte(writesKey(agencyID, accountID)))
	return &h.gates[sum.Sum32()%holdGates]
}

func (h *requestHolds) held(agencyID, accountID, roleID string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.counts[holdKey{agencyID, accountID, roleID}]
}

func (h *requestHolds) add(agencyID, accountID, roleID string, n int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.counts == nil {
		h.counts = map[holdKey]int{}
	}
	h.counts[holdKey{agencyID, accountID, ""}] += n
	if roleID != "" {
		h.counts[holdKey{agencyID, accountID, roleID}] += n
	}
}

type RequestHold struct {
	m        *accountManager
	agencyID string
	check    CheckInput
	finished atomic.Bool
}

func (m *accountManager) HoldRequest(ctx context.Context, agencyID string, in CheckInput) (Decision, *RequestHold, error) {
	gate := m.holds.gate(agencyID, in.AccountID)
	gate.Lock()
	defer gate.Unlock()
	d, err := m.checkLimits(ctx, agencyID, in, func(roleID string) int {
		return m.holds.held(agencyID, in.AccountID, roleID)
	})
	if err != nil || !d.Allowed {
		return d, nil, err
	}
	m.holds.add(agencyID, in.AccountID, in.RoleID, 1)
	return d, &RequestHold{m: m, agencyID: agencyID, check: in}, nil
}

func (h *RequestHold) Release() {
	if h.finished.CompareAndSwap(false, true) {
		h.m.holds.add(h.agencyID, h.check.AccountID, h.check.RoleID, -1)
	}
}

func (h *RequestHold) Settle(ctx context.Context, in UsageInput) (Usage, error) {
	if h.finished.Load() {
		return Usage{}, nil
	}
	gate := h.m.holds.gate(h.agencyID, h.check.AccountID)
	gate.Lock()
	defer gate.Unlock()
	defer h.Release()
	in.AccountID, in.RoleID = h.check.AccountID, h.check.RoleID
	return h.m.RecordUsage(ctx, h.agencyID, in)
}
