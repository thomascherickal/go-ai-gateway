package provider

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read fail") }
func (errReader) Close() error             { return nil }

func okClient(body string) *http.Client {
	return &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(body))}, nil
	})}
}

func TestProviderHTTPMockFactoryAndMulti(t *testing.T) {
	ctx := context.Background()
	// doJSONPost: nil client (bad URL), marshal error (chan), NewRequest error, client.Do error, io.ReadAll error
	if _, err := doJSONPost(ctx, nil, "://bad", nil, map[string]any{}, "p"); err == nil {
		t.Fatal("expected bad URL error")
	}
	if _, err := doJSONPost(ctx, http.DefaultClient, "http://x", nil, make(chan int), "p"); err == nil {
		t.Fatal("expected marshal error")
	}
	failDo := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("net") })}
	if _, err := doJSONPost(ctx, failDo, "http://x", nil, map[string]any{}, "p"); err == nil {
		t.Fatal("expected transport error")
	}
	failRead := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: errReader{}}, nil
	})}
	if _, err := doJSONPost(ctx, failRead, "http://x", nil, map[string]any{}, "p"); err == nil {
		t.Fatal("expected read error")
	}

	// MockProvider custom Handler
	mp := &MockProvider{Handler: func(context.Context, router.ModelSpec, types.ChatCompletionRequest) (Result, error) {
		return Result{Content: "custom"}, nil
	}}
	if r, err := mp.Chat(ctx, router.ModelSpec{}, types.ChatCompletionRequest{}); err != nil || r.Content != "custom" {
		t.Fatalf("MockProvider.Handler: %+v %v", r, err)
	}

	// Factory unconfigured branches
	emptyEnv := func(string) string { return "" }
	for _, v := range []string{router.ProviderAnthropic, router.ProviderGemini, router.ProviderOllama, router.ProviderOpenAICompat, router.ProviderOpenAI} {
		if BuildVendorProvider(v, emptyEnv, nil) != nil {
			t.Fatalf("expected nil for unconfigured %s", v)
		}
	}

	// MultiProvider Name & resolve branches
	if (&MultiProvider{}).Name() != "none" {
		t.Fatal("expected none")
	}
	mCompat := &MultiProvider{OpenAICompat: &MockProvider{}}
	if mCompat.Name() != "mock-local" || !mCompat.HasProvider(router.ProviderOpenAICompat) {
		t.Fatal("expected OpenAICompat resolve")
	}
	mMap := &MultiProvider{Providers: map[string]Provider{"x": &MockProvider{}}}
	if !mMap.HasProvider("x") {
		t.Fatal("expected map provider resolve")
	}
}
