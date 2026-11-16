//go:build js && wasm

package main

import (
	"database/sql"

	"github.com/syumai/workers-go/cloudflare"
	_ "github.com/syumai/workers-go/cloudflare/d1"
	"github.com/syumai/workers-go/cloudflare/fetch"

	"github.com/thomas/go-ai-gateway/internal/gateway"
	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
)

func buildGateway() *gateway.Gateway {
	d1Store := store.NewD1Store(func() (*sql.DB, error) {
		return sql.Open("d1", "DB")
	})

	prov := &provider.MultiProvider{
		WorkersAI: &provider.WorkersAIJSProvider{BindingName: "AI"},
		HasFunc: func(name string) bool {
			return name == router.ProviderWorkersAI || provider.IsVendorConfigured(name, cloudflare.Getenv)
		},
		DynamicLookup: func(name string) provider.Provider {
			client := fetch.NewClient().HTTPClient(fetch.RedirectModeFollow)
			return provider.BuildVendorProvider(name, cloudflare.Getenv, client)
		},
	}

	return gateway.New(gateway.Config{
		Store:    d1Store,
		Router:   router.New(router.Options{IsProviderAvailable: prov.HasProvider}),
		Provider: prov,
		AdminTokenFunc: func() string {
			return cloudflare.Getenv("ADMIN_TOKEN")
		},
		ProviderLimitsFunc: func() string {
			return cloudflare.Getenv("PROVIDER_LIMITS_USD")
		},
	})
}
