package store

import (
	"context"
	"sort"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func (m *MemoryStore) RecordAudit(_ context.Context, entry types.AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = m.nowFunc()
	}
	if len(entry.PIIFindings) > 0 {
		cp := make(map[string]int, len(entry.PIIFindings))
		for k, v := range entry.PIIFindings {
			cp[k] = v
		}
		entry.PIIFindings = cp
	}
	m.audits = append(m.audits, entry)
	return nil
}

func (m *MemoryStore) ListAudit(_ context.Context, filter types.AuditFilter) ([]types.AuditEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := normalizeLimit(filter.Limit)
	out := make([]types.AuditEntry, 0, limit)
	for i := len(m.audits) - 1; i >= 0; i-- {
		a := m.audits[i]
		if filter.TeamID != "" && a.TeamID != filter.TeamID {
			continue
		}
		if filter.Status != "" && a.Status != filter.Status {
			continue
		}
		if filter.Model != "" && a.RoutedModel != filter.Model && a.RequestedModel != filter.Model {
			continue
		}
		if filter.PIIOnly && a.PIIDetected == 0 {
			continue
		}
		out = append(out, a)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

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

