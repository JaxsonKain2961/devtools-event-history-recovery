package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"
)

type event struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

type ledger struct {
	mu          sync.Mutex
	seen        map[string]bool
	counts      map[string]int
	lastRedrive map[string]time.Time
	interval    time.Duration
	now         func() time.Time
}

func newLedger(interval time.Duration) *ledger {
	return &ledger{seen: map[string]bool{}, counts: map[string]int{}, lastRedrive: map[string]time.Time{}, interval: interval, now: time.Now}
}

func (l *ledger) accept(e event) (bool, error) {
	if e.ID == "" || (e.Kind != "build" && e.Kind != "release" && e.Kind != "diagnostic") {
		return false, errors.New("event needs an id and a build, release, or diagnostic kind")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.seen[e.ID] {
		return false, nil
	}
	l.seen[e.ID] = true
	l.counts[e.Kind]++
	return true, nil
}

func (l *ledger) reserve(queue string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.now().Sub(l.lastRedrive[queue]) < l.interval {
		return false
	}
	l.lastRedrive[queue] = l.now()
	return true
}

func (l *ledger) release(queue string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.lastRedrive, queue)
}

func reply(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func upstreamError(w http.ResponseWriter, err error) {
	var api *apiError
	if errors.As(err, &api) && api.Status >= 400 && api.Status < 500 {
		reply(w, api.Status, map[string]any{"error": json.RawMessage(api.Detail)})
		return
	}
	reply(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
}

func routes(client *infraiClient, l *ledger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /history", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("webhook_id")
		if !validSegment(id) {
			http.Error(w, "webhook_id required", http.StatusBadRequest)
			return
		}
		data, err := client.history(r.Context(), id)
		if err != nil {
			upstreamError(w, err)
			return
		}
		reply(w, http.StatusOK, map[string]any{"deliveries": json.RawMessage(data)})
	})
	mux.HandleFunc("POST /events", func(w http.ResponseWriter, r *http.Request) {
		var e event
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&e); err != nil {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}
		accepted, err := l.accept(e)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		reply(w, http.StatusOK, map[string]any{"accepted": accepted, "kind": e.Kind})
	})
	mux.HandleFunc("POST /redrive", func(w http.ResponseWriter, r *http.Request) {
		queue, id := r.URL.Query().Get("queue"), r.Header.Get("Idempotency-Key")
		if !validSegment(queue) || id == "" {
			http.Error(w, "queue and Idempotency-Key required", http.StatusBadRequest)
			return
		}
		if !l.reserve(queue) {
			reply(w, http.StatusTooManyRequests, map[string]string{"error": "redrive interval active"})
			return
		}
		data, err := client.redrive(r.Context(), queue, id)
		if err != nil {
			l.release(queue)
			upstreamError(w, err)
			return
		}
		reply(w, http.StatusOK, map[string]any{"redrive": json.RawMessage(data)})
	})
	return mux
}
