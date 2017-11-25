package sleep

import (
	"testing"
	"runtime"
	"math/rand"
	"time"
)

// TestBlock tests that a sleeper actually blocks waiting for the waker to
// assert its state
func TestBlock(t *testing.T) {
	var w Waker
	var s Sleeper

	s.AddWaker(&w, 0)

	// Assert waker after one second
	before := time.Now()
	go func() {
		time.Sleep(1 * time.Second)
		w.Assert()
	}()

	// Fetch the result and make sure it took at least 500ms
	if _, ok := s.Fetch(true); !ok {
		t.Fatalf("Fetch failed unexpectedly")
	}

	if d := time.Now().Sub(before); d < 500 * time.Millisecond {
		t.Fatalf("Duration was too short: %v", d)
	}

	// Check that already-asserted waker completes inline
	w.Assert()
	if _, ok := s.Fetch(true); !ok {
		t.Fatalf("Fetch failed unexpectedly")
	}

	// Check that fetch sleeps if waker had been asserted but was reset
	// before Fetch is called
	w.Assert()
	w.Clear()
	before = time.Now()
	go func() {
		time.Sleep(1 * time.Second)
		w.Assert()
	}()
	if _, ok := s.Fetch(true); !ok {
		t.Fatalf("Fetch failed unexpectedly")
	}
	if d := time.Now().Sub(before); d < 500 * time.Millisecond {
		t.Fatalf("Duration was too short: %v", d)
	}
}

// TestNonBlock checks that a sleeper won't block if waker isn't asserted
func TestNonBlock(t *testing.T) {
	var w Waker
	var s Sleeper

	// Don't block when there's no waker
	if _, ok := s.Fetch(false); ok {
		t.Fatalf("Fetch succeeded when there is no waker")
	}

	// Don't block when waker isn't asserted
	s.AddWaker(&w, 0)
	if _, ok := s.Fetch(false); ok {
		t.Fatalf("Fetch succeeded when waker was not asserted")
	}

	// Don't block when waker was asserted, but isn't anymore
	w.Assert()
	w.Clear()
	if _, ok := s.Fetch(false); ok {
		t.Fatalf("Fetch succeeded when waker was not asserted anymore")
	}

	// Don't block when waker was consumed by previous Fetch()
	w.Assert()
	if _, ok := s.Fetch(false); !ok {
		t.Fatalf("Fetch failed even though waker was asserted")
	}

	if _, ok := s.Fetch(false); ok {
		t.Fatalf("Fetch succeeded when waker had been consumed")
	}
}

// TestMultiple checks that a sleeper can wait for and receives notifications
// from multiple wakers
func TestMultiple(t *testing.T) {
	var s Sleeper
	var w1, w2 Waker

	s.AddWaker(&w1, 0)
	s.AddWaker(&w2, 1)

	w1.Assert()
	w2.Assert()

	v, ok := s.Fetch(false)
	if !ok {
		t.Fatalf("Fetch failed when there are asserted wakers")
	}

	if v != 0 && v != 1 {
		t.Fatalf("Unexpectedly waker id: %v", v)
	}

	want := 1 - v
	v, ok = s.Fetch(false)
	if !ok {
		t.Fatalf("Fetch failed when there is an asserted waker")
	}

	if v != want {
		t.Fatalf("Unexpected waker id, got %v, want %v", v, want)
	}
}

// TestDoneFunction tests if calling Done() on a sleeper works properly
func TestDoneFunction(t *testing.T) {
	// Trival case of no waker
	var s Sleeper
	s.Done()

	// Cases when.the sleeper has n wakers, but none are asserted
	for n := 1; n < 20; n++ {
		var s Sleeper
		w := make([]Waker, n)
		for j := 0; j < n; j++ {
			s.AddWaker(&w[j], j)
		}
		s.Done()
	}

	// Cases when the sleeper has n wakers, and only the i-th one is
	// asserted
	for n := 1; n < 20; n++ {
		for i := 0; i < n; i++ {
			var s Sleeper
			w := make([]Waker, n)
			for j := 0; j < n; j++ {
				s.AddWaker(&w[j], j)
			}
			w[i].Assert()
			s.Done()
		}
	}

	// Cases when the sleeper has n wakers, and the i-th one is asserted
	// and cleared
	for n := 1; n < 20; n++ {
		for i := 0; i < n; i++ {
			var s Sleeper
			w := make([]Waker, n)
			for j := 0; j < n; j++ {
				s.AddWaker(&w[j], j)
			}
			w[i].Assert()
			w[i].Clear()
			s.Done()
		}
	}

	// Cases when the sleeper has n wakers, with a random number of them
	// asserted
	for n := 1; n < 20; n++ {
		for iters := 0; iters < 1000; iters++ {
			var s Sleeper
			w := make([]Waker, n)
			for j := 0; j < n; j++ {
				s.AddWaker(&w[j], j)
			}

			// Pick the number of associated elements, then assert
			// random wakers
			asserted := rand.Int() % (n + 1)
			for j := 0; j < asserted; j++ {
				w[rand.Int() % n].Assert()
			}
			s.Done()
		}
	}
}

// TestRace tests that multiple wakers can continuously send wake requests to
// the sleeper
func TestRace(t *testing.T) {
	const wakers = 100
	const wakeRequests = 10000

	counts := make([]int, wakers)
	w := make([]Waker, wakers)
	var s Sleeper

	// Associate each waker and start goroutines that will assert them
	for i := range w {
		s.AddWaker(&w[i], i)
		go func(w *Waker) {
			n := 0
			for n < wakeRequests {
				if !w.IsAsserted() {
					w.Assert()
					n++
				} else {
					runtime.Gosched()
				}
			}
		}(&w[i])
	}

	// Wait for all wake up notifications from all wakers
	for i := 0; i < wakers * wakeRequests; i++ {
		v, _ := s.Fetch(true)
		counts[v]++
	}

	// Check that we got the right number for each
	for i, v := range counts {
		if v != wakeRequests {
			t.Errorf("Waker %v only got %v wakes", i, v)
		}
	}
}
