package jobs

import (
	"sync"
	"testing"
)

func TestRunnerQueuesAndRunsConcurrentTargets(t *testing.T) {
	r := NewRunner(nil, nil, t.TempDir(), 2)

	barrier := make(chan struct{})
	started := make(chan struct{}, 3)
	var wg sync.WaitGroup

	enqueue := func(target string) {
		wg.Add(1)
		r.mu.Lock()
		r.queue = append(r.queue, queuedWork{
			jobID:     target,
			targetKey: target,
			run: func() {
				defer wg.Done()
				started <- struct{}{}
				<-barrier
			},
		})
		r.pumpLocked()
		r.mu.Unlock()
	}

	enqueue("a")
	enqueue("b")
	enqueue("c")

	<-started
	<-started
	r.mu.Lock()
	queued := len(r.queue)
	r.mu.Unlock()
	if queued != 1 {
		t.Fatalf("expected third job queued, got %d queued", queued)
	}
	close(barrier)
	wg.Wait()
}

func TestRunnerSerializesSameTarget(t *testing.T) {
	r := NewRunner(nil, nil, t.TempDir(), 4)

	var order []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	ready := make(chan struct{})

	run := func(name string) {
		wg.Add(1)
		r.mu.Lock()
		r.queue = append(r.queue, queuedWork{
			jobID:     name,
			targetKey: "same",
			run: func() {
				defer wg.Done()
				mu.Lock()
				order = append(order, name+"-start")
				mu.Unlock()
				<-ready
				mu.Lock()
				order = append(order, name+"-end")
				mu.Unlock()
			},
		})
		r.pumpLocked()
		r.mu.Unlock()
	}

	run("first")
	run("second")
	close(ready)
	wg.Wait()

	want := []string{"first-start", "first-end", "second-start", "second-end"}
	if len(order) != len(want) {
		t.Fatalf("order %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order %v, want %v", order, want)
		}
	}
}
