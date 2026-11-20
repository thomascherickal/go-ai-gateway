package gateway

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func (g *Gateway) newRequestID() string {
	n := g.seq.Add(1)
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("gw_%d_%x_%d", g.now().UnixMilli(), b, n)
}

func readLimitedBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes))
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "failed to read request body", "")
		return nil, false
	}
	return body, true
}

func extractFirstKey(r *http.Request, headers ...string) string {
	for _, h := range headers {
		if v := strings.TrimSpace(r.Header.Get(h)); v != "" {
			return v
		}
	}
	return extractBearerOrHeader(r, "")
}

func extractBearerOrHeader(r *http.Request, altHeader string) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if altHeader != "" {
		return strings.TrimSpace(r.Header.Get(altHeader))
	}
	return ""
}

func hashMessages(msgs []types.ChatMessage) string {
	h := sha256.New()
	for _, m := range msgs {
		h.Write([]byte(m.Role))
		h.Write([]byte{0})
		h.Write([]byte(m.Content))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func elapsedMs(now, start time.Time) int64 {
	if ms := now.Sub(start).Milliseconds(); ms >= 0 {
		return ms
	}
	return 0
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeOpenAIError(w http.ResponseWriter, status int, errType, message, code string) {
	writeJSON(w, status, types.ErrorResponse{
		Error: types.ErrorDetail{Message: message, Type: errType, Code: code},
	})
}
