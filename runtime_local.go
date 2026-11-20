//go:build !js

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thomas/go-ai-gateway/internal/config"
	"github.com/thomas/go-ai-gateway/internal/gateway"
	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/store"
)

func buildGateway() *gateway.Gateway {
	loadedEnv := config.LoadEnvFiles(config.DefaultPersonalEnvPaths()...)
	personalMode := strings.EqualFold(os.Getenv("PERSONAL_MODE"), "true")

	st := initLocalStore(personalMode)
	if personalMode {
		bootstrapPersonalProfile(st)
	}

	httpClient := &http.Client{Timeout: 45 * time.Second}
	adminToken := os.Getenv("ADMIN_TOKEN")
	if adminToken == "" {
		adminToken = "gw_admin_dev"
		fmt.Fprintln(os.Stderr, "info: ADMIN_TOKEN not set; using local dev token 'gw_admin_dev'")
	}

	prov, activeNames := initLocalProviders(httpClient)
	fmt.Fprintf(os.Stderr, "info: local gateway initialized (mode=%s, store=%s, providers=%s, env_files=%v)\n",
		modeLabel(personalMode), st.BackendName(), strings.Join(activeNames, ","), loadedEnv)

	return gateway.New(gateway.Config{
		Store:              st,
		Router:             router.New(router.Options{IsProviderAvailable: prov.HasProvider}),
		Provider:           prov,
		AdminTokenFunc:     func() string { return adminToken },
		ProviderLimitsFunc: func() string { return os.Getenv("PROVIDER_LIMITS_USD") },
	})
}

var userHomeDir = os.UserHomeDir

func initLocalStore(personalMode bool) store.Store {
	dataFile := strings.TrimSpace(os.Getenv("GATEWAY_DATA_FILE"))
	if dataFile == "" && personalMode {
		if home, err := userHomeDir(); err == nil && home != "" {
			dataFile = filepath.Join(home, ".config", "go-ai-gateway", "state.json")
		} else {
			dataFile = "gateway-state.json"
		}
	}
	if dataFile != "" {
		fs, err := store.NewFileStore(dataFile, !personalMode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: failed to open GATEWAY_DATA_FILE=%s (%v); falling back to memory store\n", dataFile, err)
			return store.NewMemoryStore(!personalMode)
		}
		return fs
	}
	return store.NewMemoryStore(!personalMode)
}

func modeLabel(personal bool) string {
	if personal {
		return "personal"
	}
	return "enterprise"
}
