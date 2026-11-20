package router

import "github.com/thomas/go-ai-gateway/internal/types"

func (r *Router) findAvailableFallback(primary ModelSpec, maxTier types.ModelTier) (ModelSpec, bool) {
	if chain := r.walkChain(primary, maxTier, true); len(chain) > 0 {
		return chain[0], true
	}
	if r.isAvailable(ProviderWorkersAI) {
		tier := primary.Tier
		if maxTier != "" && types.TierRank(tier) > types.TierRank(maxTier) {
			tier = maxTier
		}
		return r.tierDefaultModel(tier, ""), true
	}
	return ModelSpec{}, false
}

func (r *Router) walkChain(primary ModelSpec, maxTier types.ModelTier, firstOnly bool) []ModelSpec {
	var chain []ModelSpec
	seen := map[string]bool{primary.ID: true}
	cur := primary
	for cur.FallbackID != "" {
		if seen[cur.FallbackID] {
			break
		}
		seen[cur.FallbackID] = true
		fb, ok := r.models[cur.FallbackID]
		if !ok {
			break
		}
		if r.isAvailable(fb.Provider) && (maxTier == "" || types.TierRank(fb.Tier) <= types.TierRank(maxTier)) {
			chain = append(chain, fb)
			if firstOnly {
				return chain
			}
		}
		cur = fb
	}
	return chain
}
