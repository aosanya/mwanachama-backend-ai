package ai

import (
	"sync"
	"testing"
	"time"
)

func TestAWriteReturnsAtOnceAndASettledReadWaitsForIt(t *testing.T) {
	var u usageWrites
	release := make(chan struct{})
	u.after("ag", "acc", func() { <-release })
	returned := make(chan struct{})

	go func() {
		u.settled("ag", "acc")
		close(returned)
	}()

	select {
	case <-returned:
		t.Fatal("a read was settled before the write finished")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("a read never settled after the write finished")
	}
}

func TestWritesOfOneAccountRunInOrderAndAnotherAccountIsNotHeld(t *testing.T) {
	var u usageWrites
	var mu sync.Mutex
	var order []int
	release := make(chan struct{})
	u.after("ag", "acc", func() {
		<-release
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
	})
	u.after("ag", "acc", func() {
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()
	})

	otherSettled := make(chan struct{})
	go func() {
		u.settled("ag", "other")
		close(otherSettled)
	}()
	select {
	case <-otherSettled:
	case <-time.After(time.Second):
		t.Fatal("another account was held by the pending writes")
	}
	close(release)
	u.settled("ag", "acc")

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("order %v", order)
	}
}
