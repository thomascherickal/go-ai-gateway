package gateway

import (
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
	"github.com/thomas/go-ai-gateway/internal/types"
)

const maxRequestBodyBytes = 2 << 20 // 2 MiB

// Config wires dependencies and runtime configuration into the HTTP gateway.
type Config struct {
	Store              store.Store
	Router             *router.Router
	Provider           provider.Provider
	AdminTokenFunc     func() string
	ProviderLimitsFunc func() string
	NowFunc            func() time.Time
}

// Gateway is the HTTP AI traffic control plane handler.
type Gateway struct {
	store        store.Store
	router       *router.Router
	provider     provider.Provider
	adminToken   func() string
	providerCaps *ProviderCapTracker
	now          func() time.Time
	seq          atomic.Uint64
	mux          *http.ServeMux
}

// New constructs a Gateway and registers all control-plane routes.
func New(cfg Config) *Gateway {
	nowFn := cfg.NowFunc
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	adminFn := cfg.AdminTokenFunc
	if adminFn == nil {
		adminFn = func() string { return "" }
	}

	g := &Gateway{
		store:        cfg.Store,
		router:       cfg.Router,
		provider:     cfg.Provider,
		adminToken:   adminFn,
		providerCaps: newProviderCapTracker(cfg.ProviderLimitsFunc),
		now:          nowFn,
		mux:          http.NewServeMux(),
	}

	g.mux.HandleFunc("/", g.handleRoot)
	g.mux.HandleFunc("/healthz", g.handleHealth)
	g.mux.HandleFunc("/v1/models", g.handleModels)
	g.mux.HandleFunc("/v1/models/", g.handleGeminiModels)
	g.mux.HandleFunc("/v1beta/models", g.handleGeminiModelList)
	g.mux.HandleFunc("/v1beta/models/", g.handleGeminiModels)
	g.mux.HandleFunc("/v1/messages/count_tokens", g.handleAnthropicCountTokens)
	g.mux.HandleFunc("/v1/messages", g.handleAnthropicMessages)
	g.mux.HandleFunc("/v1/chat/completions", g.handleChatCompletions)
	g.mux.HandleFunc("/admin/teams", g.requireAdmin(g.handleAdminTeams))
	g.mux.HandleFunc("/admin/teams/", g.requireAdmin(g.handleAdminTeamAction))
	g.mux.HandleFunc("/admin/audit", g.requireAdmin(g.handleAdminAudit))
	g.mux.HandleFunc("/admin/summary", g.requireAdmin(g.handleAdminSummary))
	g.mux.HandleFunc("/admin/providers", g.requireAdmin(g.handleAdminProviders))
	return g
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Team-Key, X-Admin-Token, X-Tool-Name, X-Gateway-Route, X-Gateway-Provider, X-Gateway-Strict-Provider, x-api-key, anthropic-version, x-goog-api-key")
	w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, X-Gateway-Protocol, X-Gateway-Team, X-Gateway-Routed-Model, X-Gateway-Provider, X-Gateway-PII-Action, X-Gateway-PII-Detected, X-Gateway-Cost-Micros, X-Gateway-Remaining-Micros")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	g.mux.ServeHTTP(w, r)
}

func (g *Gateway) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeOpenAIError(w, http.StatusNotFound, "not_found", "route not found", "")
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		writeJSON(w, http.StatusOK, map[string]any{
			"service":  "go-ai-gateway",
			"store":    g.store.BackendName(),
			"provider": g.provider.Name(),
			"endpoints": []string{
				"POST /v1/chat/completions",
				"POST /v1/messages",
				"POST /v1beta/models/{model}:generateContent",
				"GET /v1/models",
				"GET /healthz",
				"GET|POST /admin/teams",
				"GET /admin/audit",
				"GET /admin/summary",
			},
		})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(dashboardHTML))
}

func (g *Gateway) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"store":    g.store.BackendName(),
		"provider": g.provider.Name(),
		"time":     g.now().Format(time.RFC3339),
	})
}

func (g *Gateway) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed", "")
		return
	}
	specs := g.router.Catalog()
	type modelEntry struct {
		ID                string          `json:"id"`
		Object            string          `json:"object"`
		OwnedBy           string          `json:"owned_by"`
		Tier              types.ModelTier `json:"tier"`
		Aliases           []string        `json:"aliases,omitempty"`
		InputMicrosPer1M  int64           `json:"input_micros_per_1m"`
		OutputMicrosPer1M int64           `json:"output_micros_per_1m"`
		Description       string          `json:"description"`
	}
	data := make([]modelEntry, 0, len(specs)+1)
	data = append(data, modelEntry{
		ID:          "auto",
		Object:      "model",
		OwnedBy:     "go-ai-gateway",
		Tier:        types.TierBalanced,
		Description: "Automatic complexity, code, and cost-aware router",
	})
	for _, s := range specs {
		data = append(data, modelEntry{
			ID:                s.ID,
			Object:            "model",
			OwnedBy:           s.Provider,
			Tier:              s.Tier,
			Aliases:           s.Aliases,
			InputMicrosPer1M:  s.InputMicrosPer1M,
			OutputMicrosPer1M: s.OutputMicrosPer1M,
			Description:       s.Description,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}
