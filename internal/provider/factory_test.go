package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func TestVendorFactoryAndMultiProvider(t *testing.T) {
	env := map[string]string{
		"ANTHROPIC_API_KEY": "k1",
		"GEMINI_API_KEY":    "k2",
		"XAI_API_KEY":       "k3",
		"DEEPSEEK_API_KEY":  "k4",
		"QWEN_API_KEY":      "k5",
		"OPENAI_API_KEY":    "k6",
		"MISTRAL_API_KEY":   "k7",
		"OLLAMA_BASE_URL":   "http://localhost:11434/v1",
		"UPSTREAM_BASE_URL": "http://localhost:8000/v1",
	}
	getenv := func(k string) string { return env[k] }
	for _, v := range ExternalVendors {
		if !IsVendorConfigured(v, getenv) {
			t.Fatalf("expected %s configured", v)
		}
		if p := BuildVendorProvider(v, getenv, nil); p == nil || p.Name() == "" {
			t.Fatalf("BuildVendorProvider(%s) returned nil", v)
		}
	}
	if IsVendorConfigured("unknown", getenv) || BuildVendorProvider("unknown", getenv, nil) != nil {
		t.Fatalf("expected unknown vendor to be nil")
	}

	mock := &MockProvider{}
	mp := &MultiProvider{WorkersAI: mock}
	if mp.Name() != "mock-local" || !mp.HasProvider(router.ProviderWorkersAI) || mp.HasProvider("missing") {
		t.Fatalf("unexpected MultiProvider state")
	}
	res, err := mp.Chat(context.Background(), router.ModelSpec{ID: "m1", Provider: router.ProviderWorkersAI}, types.ChatCompletionRequest{
		Messages: []types.ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil || res.Content == "" {
		t.Fatalf("mock chat failed: %+v, err=%v", res, err)
	}
}

func TestWorkersAIRESTProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"result":  map[string]any{"response": "edge ok"},
		})
	}))
	defer srv.Close()

	p := &WorkersAIRESTProvider{AccountID: "", APIToken: ""}
	if _, err := p.Chat(context.Background(), router.ModelSpec{ID: "m"}, types.ChatCompletionRequest{}); err == nil {
		t.Fatalf("expected error on missing credentials")
	}
	if p.Name() != "workers-ai-rest" {
		t.Fatalf("unexpected name: %s", p.Name())
	}
}
