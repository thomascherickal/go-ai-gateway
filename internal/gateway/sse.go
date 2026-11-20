package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func startSSE(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
}

func writeSSEData(w http.ResponseWriter, eventName string, payload any) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if eventName != "" {
		_, _ = fmt.Fprintf(w, "event: %s\n", eventName)
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func splitSSEChunks(s string, chunkSize int) []string {
	if s == "" {
		return nil
	}
	if chunkSize <= 0 {
		return []string{s}
	}
	var chunks []string
	runes := []rune(s)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}
