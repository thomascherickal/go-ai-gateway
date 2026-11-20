//go:build !js

package main

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/provider"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func bootstrapPersonalProfile(st store.Store) {
	apiKey := strings.TrimSpace(os.Getenv("PERSONAL_API_KEY"))
	if apiKey == "" {
		apiKey = "gw_personal_dev"
		fmt.Fprintln(os.Stderr, "info: PERSONAL_API_KEY not set; using local personal key 'gw_personal_dev'")
	}
	budgetUSD := 25.0
	if raw := strings.TrimSpace(os.Getenv("PERSONAL_MONTHLY_BUDGET_USD")); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed > 0 {
			budgetUSD = parsed
		}
	}
	piiMode := types.PIIMode(strings.TrimSpace(os.Getenv("PERSONAL_PII_MODE")))
	if piiMode == "" {
		piiMode = types.PIIModeRedact
	}
	maxTier := types.ModelTier(strings.TrimSpace(os.Getenv("PERSONAL_MAX_TIER")))
	if maxTier == "" {
		maxTier = types.TierReasoning
	}
	enabled := true
	_, _ = st.UpsertTeam(context.Background(), store.UpsertTeamInput{
		TeamID:             "personal",
		TeamName:           "Personal Workspace",
		RawAPIKey:          apiKey,
		MonthlyLimitMicros: int64(math.Round(budgetUSD * 1_000_000)),
		MaxTier:            maxTier,
		PIIMode:            piiMode,
		Enabled:            &enabled,
	})
}

func initLocalProviders(httpClient *http.Client) (*provider.MultiProvider, []string) {
	providers := map[string]provider.Provider{}
	var activeNames []string

	if cfAcc, cfTok := os.Getenv("CF_ACCOUNT_ID"), os.Getenv("CF_API_TOKEN"); cfAcc != "" && cfTok != "" {
		providers[router.ProviderWorkersAI] = &provider.WorkersAIRESTProvider{
			AccountID:  cfAcc,
			APIToken:   cfTok,
			HTTPClient: httpClient,
		}
		activeNames = append(activeNames, "workers-ai-rest")
	}
	for _, name := range provider.ExternalVendors {
		if p := provider.BuildVendorProvider(name, os.Getenv, httpClient); p != nil {
			providers[name] = p
			activeNames = append(activeNames, name)
		}
	}

	mock := &provider.MockProvider{}
	if len(providers) == 0 {
		for _, name := range provider.AllVendors {
			if name != router.ProviderOpenAICompat && name != router.ProviderOllama {
				providers[name] = mock
			}
		}
		activeNames = append(activeNames, "mock-all-vendors")
	} else if providers[router.ProviderWorkersAI] == nil {
		providers[router.ProviderWorkersAI] = mock
		activeNames = append(activeNames, "workers-ai-mock")
	}
	return &provider.MultiProvider{Providers: providers}, activeNames
}
