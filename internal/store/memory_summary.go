package store

import (
	"context"
	"sort"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func (m *MemoryStore) GetSummary(_ context.Context) (types.AuditSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := types.AuditSummary{}
	teamMap := make(map[string]*types.TeamSpendSummary, len(m.teams))
	for _, t := range m.teams {
		teamMap[t.TeamID] = &types.TeamSpendSummary{
			TeamID:             t.TeamID,
			TeamName:           t.TeamName,
			MonthlyLimitMicros: t.MonthlyLimitMicros,
			SpentMicros:        t.SpentMicros,
		}
	}

	for _, a := range m.audits {
		summary.TotalRequests++
		summary.TotalPIIDetected += int64(a.PIIDetected)
		summary.TotalTokens += int64(a.TotalTokens)
		summary.TotalCostMicros += a.CostMicros

		switch a.Status {
		case types.StatusOK:
			summary.SuccessfulCount++
		case types.StatusBlockedBudget:
			summary.BlockedBudget++
		case types.StatusBlockedPII:
			summary.BlockedPII++
		case types.StatusUpstreamError:
			summary.UpstreamErrors++
		}

		ts, ok := teamMap[a.TeamID]
		if !ok {
			ts = &types.TeamSpendSummary{TeamID: a.TeamID}
			teamMap[a.TeamID] = ts
		}
		ts.RequestCount++
		ts.PIIDetections += int64(a.PIIDetected)
		if a.Status == types.StatusBlockedBudget || a.Status == types.StatusBlockedPII || a.Status == types.StatusBlockedPolicy {
			ts.BlockedCount++
		}
	}

	teams := make([]types.TeamSpendSummary, 0, len(teamMap))
	for _, ts := range teamMap {
		teams = append(teams, *ts)
	}
	sort.Slice(teams, func(i, j int) bool { return teams[i].TeamID < teams[j].TeamID })
	summary.Teams = teams
	return summary, nil
}
