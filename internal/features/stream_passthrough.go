package features

import (
	"bufio"
	"io"
	"net/http"
)

// StreamPassThrough copies an upstream SSE stream directly to w with immediate flushing.
func StreamPassThrough(w http.ResponseWriter, upstream io.Reader) error {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	scanner := bufio.NewScanner(upstream)
	for scanner.Scan() {
		if _, err := w.Write(append(scanner.Bytes(), '\n')); err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
	return scanner.Err()
}
