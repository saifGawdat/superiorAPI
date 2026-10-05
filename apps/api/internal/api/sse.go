package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	progressInterval  = 200 * time.Millisecond
	heartbeatInterval = 15 * time.Second
)

// streamTest sends `progress` events while a test runs and a final `done`
// event with the full result.
func (s *Server) streamTest(w http.ResponseWriter, r *http.Request) {
	t, ok := s.lookup(w, r)
	if !ok {
		return
	}
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering
	w.WriteHeader(http.StatusOK)

	send := func(event string, v any) bool {
		data, err := json.Marshal(v)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
			return false
		}
		return rc.Flush() == nil
	}

	progress, version := t.Progress()
	select {
	case <-t.Done():
		send("done", t.Snapshot())
		return
	default:
		if !send("progress", progress) {
			return
		}
	}

	ticker := time.NewTicker(progressInterval)
	defer ticker.Stop()
	lastWrite := time.Now()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-t.Done():
			send("done", t.Snapshot())
			return
		case <-ticker.C:
			p, v := t.Progress()
			if v != version {
				version = v
				if !send("progress", p) {
					return
				}
				lastWrite = time.Now()
			} else if time.Since(lastWrite) >= heartbeatInterval {
				if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil || rc.Flush() != nil {
					return
				}
				lastWrite = time.Now()
			}
		}
	}
}
