package store

import (
	"context"

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
