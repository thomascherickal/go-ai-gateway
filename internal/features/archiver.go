package features

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/thomas/go-ai-gateway/internal/types"
)

// SelectReplicaRegion picks the closest D1 read-replica region for a Cloudflare colo/region hint.
func SelectReplicaRegion(cfRegion string) string {
	switch strings.ToUpper(strings.TrimSpace(cfRegion)) {
	case "WNAM", "ENAM":
		return "nam"
	case "WEUR", "EEUR":
		return "eur"
	case "APAC":
		return "apac"
	default:
		return "primary"
	}
}

// ArchiveAuditNDJSON writes audit entries as newline-delimited JSON to an R2-compatible writer.
func ArchiveAuditNDJSON(w io.Writer, entries []types.AuditEntry) (int, error) {
	var buf bytes.Buffer
	for _, e := range entries {
		b, _ := json.Marshal(e)
		buf.Write(b)
		buf.WriteByte('\n')
	}
	n, err := w.Write(buf.Bytes())
	return n, err
}
