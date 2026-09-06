package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

// stores incoming reqs in entered and blocks untill release is closed
type blockingHandler struct {
	release chan struct{}
	entered chan struct{}
}

func newBlockingHandler() *blockingHandler {
	return &blockingHandler{
		release: make(chan struct{}),
		entered: make(chan struct{}, 64),
	}
}

func (h *blockingHandler) serve(w http.ResponseWriter, _ *http.Request) {
	h.entered <- struct{}{}
	// req will block here
	<-h.release
	w.WriteHeader(http.StatusOK)
}

func TestLimitConcurrentServesItsLimit(t *testing.T) {
	for _, limit := range []int{1, 2, 10} {
		h := newBlockingHandler()
		// limited request would've waited for a minute
		limited := limitConcurrent(limit, time.Minute)(h.serve)

		// spawn limit reqs, they all block inside the h.serve (but pass the limiter)
		var wg sync.WaitGroup
		for range limit {
			wg.Go(func() {
				limited.ServeHTTP(httptest.NewRecorder(), genericRequest())
			})
		}

		// we check they actually reached h.serve (nothing waits for a minute inside the limiter)
		for range limit {
			select {
			case <-h.entered:
			case <-time.After(2 * time.Second):
				t.Fatal("a request below the limit did not reach the handler")
			}
		}

		// release blocked reqs, so goroutines will end
		close(h.release)
		wg.Wait()
	}
}

func TestLimitConcurrentRejectsOverItsLimitAfterWait(t *testing.T) {
	for _, limit := range []int{1, 2, 10} {
		h := newBlockingHandler()
		limited := limitConcurrent(limit, 20*time.Millisecond)(h.serve)

		var wg sync.WaitGroup
		for range limit {
			wg.Go(func() {
				limited.ServeHTTP(httptest.NewRecorder(), genericRequest())
			})
		}

		// block untill all slots taken
		for range limit {
			<-h.entered
		}

		rec := httptest.NewRecorder()
		limited.ServeHTTP(rec, genericRequest())

		require.Equal(t, http.StatusTooManyRequests, rec.Code)
		require.Equal(t, retryAfterBusy, rec.Header().Get("Retry-After"))
		require.JSONEq(t, `{"error":"server is busy, retry later"}`, rec.Body.String())

		close(h.release)
		wg.Wait()
	}
}

func TestLimitConcurrentReleasesTheSlotAfterTheHandlerReturns(t *testing.T) {
	limited := limitConcurrent(1, 20*time.Millisecond)(
		// non blocking handler
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

	for range 3 {
		rec := httptest.NewRecorder()
		limited.ServeHTTP(rec, genericRequest())
		require.Equal(t, http.StatusOK, rec.Code) // no 429
	}
}

func TestLimitConcurrentWaitsForAFreedSlotInsteadOfRejecting(t *testing.T) {
	h := newBlockingHandler()
	limited := limitConcurrent(1, time.Minute)(h.serve)

	// take all slots and block inside the handler
	var wg sync.WaitGroup
	wg.Go(func() {
		limited.ServeHTTP(httptest.NewRecorder(), genericRequest())
	})
	<-h.entered

	// make one more req, it blocks inside the limiter waiting for slot (for a minute)
	queued := make(chan int, 1)
	wg.Go(func() {
		rec := httptest.NewRecorder()
		limited.ServeHTTP(rec, genericRequest()) // will block here
		queued <- rec.Code
	})

	// release the first req, the queued one unblocks and takes the freed slot
	close(h.release)
	<-h.entered

	// in the end resp is OK, not 429
	select {
	case code := <-queued:
		require.Equal(t, http.StatusOK, code)
	case <-time.After(2 * time.Second):
		t.Fatal("a queued request never completed")
	}
	wg.Wait()
}

func TestLimitConcurrentWritesNothingWhenTheClientHungUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newBlockingHandler()
		limited := limitConcurrent(1, time.Minute)(h.serve)

		// take all slots and block inside the handler
		var wg sync.WaitGroup
		wg.Go(func() {
			limited.ServeHTTP(httptest.NewRecorder(), genericRequest())
		})
		<-h.entered

		// make a req with a ctx we are going to hang up
		ctx, cancel := context.WithCancel(t.Context())
		rec := httptest.NewRecorder()
		hungUp := make(chan struct{})
		wg.Go(func() {
			defer close(hungUp)
			limited.ServeHTTP(rec, genericRequest().WithContext(ctx))
		})

		// Make sure the second req is parked inside the limiter.
		synctest.Wait()
		// hang up
		cancel()
		<-hungUp // req is done, rec can now be read

		// expected: limiter blocked the second req for a minute,
		// but its ctx was done much earlier. Nothing instead of 429
		require.Empty(t, rec.Body.String())
		require.False(t, rec.Flushed)
		require.Empty(t, rec.Header().Get("Retry-After"))

		close(h.release)
		wg.Wait()
	})
}
