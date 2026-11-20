package gateway

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
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
