package gateway

import (
	"context"
	"sync"
	"time"

	"github.com/thomas/go-ai-gateway/internal/store"
	"github.com/thomas/go-ai-gateway/internal/types"
)

// ProviderCapTracker enforces per-vendor monthly spend caps across all requests,
// enabling automatic overflow to cheaper/free/local providers in multi-subscription setups.
type ProviderCapTracker struct {
	mu       sync.Mutex
	specFunc func() string
	periodYM string
	seeded   bool
	spent    map[string]int64
}

func newProviderCapTracker(specFunc func() string) *ProviderCapTracker {
	return &ProviderCapTracker{
		specFunc: specFunc,
		spent:    make(map[string]int64),
	}
}

func (p *ProviderCapTracker) ensureSeededLocked(ctx context.Context, st store.Store, now time.Time) {
	ym := store.PeriodYM(now)
	if p.periodYM != ym {
		p.periodYM = ym
		p.spent = make(map[string]int64)
		p.seeded = false
	}
	if p.seeded || st == nil {
		return
	}
	p.seeded = true
	entries, err := st.ListAudit(ctx, types.AuditFilter{Status: types.StatusOK, Limit: 200})
	if err != nil {
		return
	}
	for _, e := range entries {
		if store.PeriodYM(e.CreatedAt) == ym && e.Provider != "" && e.CostMicros > 0 {
			p.spent[e.Provider] += e.CostMicros
		}
	}
}

func (p *ProviderCapTracker) limits() map[string]int64 {
	if p.specFunc == nil {
		return nil
	}
	return ParseProviderLimitsUSD(p.specFunc())
}

// IsCapped reports whether a provider has reached its configured monthly USD cap.
func (p *ProviderCapTracker) IsCapped(ctx context.Context, st store.Store, provider string, now time.Time) bool {
	lims := p.limits()
	capMicros, hasCap := lims[provider]
	if !hasCap {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ensureSeededLocked(ctx, st, now)
	return p.spent[provider] >= capMicros
}

// RecordSpend adds confirmed spend for the provider in the current UTC month.
func (p *ProviderCapTracker) RecordSpend(ctx context.Context, st store.Store, provider string, costMicros int64, now time.Time) {
	if provider == "" || costMicros <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ensureSeededLocked(ctx, st, now)
	p.spent[provider] += costMicros
}

// Snapshot returns current-month spend and caps per provider.
func (p *ProviderCapTracker) Snapshot(ctx context.Context, st store.Store, now time.Time) (map[string]int64, map[string]int64) {
	lims := p.limits()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ensureSeededLocked(ctx, st, now)

	spentCopy := make(map[string]int64, len(p.spent))
	for k, v := range p.spent {
		spentCopy[k] = v
	}
	return spentCopy, lims
}
