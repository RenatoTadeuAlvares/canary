package edge

import "time"

// ProtectionRecord is exact local submitted-proposal provenance for a broker
// execution. It is contextual evidence, never a historical risk verdict.
type ProtectionRecord struct {
	Identity string    `json:"identity"`
	Bucket   string    `json:"bucket"`
	At       time.Time `json:"at"`
}

// ProtectionContext keeps a linked protection purpose separate from price P/L.
type ProtectionContext struct {
	Status            string    `json:"status"`
	ExecutionCount    int       `json:"execution_count"`
	MatchedExecutions int       `json:"matched_executions"`
	Bucket            string    `json:"bucket,omitempty"`
	EvidenceAt        time.Time `json:"evidence_at,omitzero"`
}

func changeProtectionContext(ids []string, records map[string]ProtectionRecord) ProtectionContext {
	out := ProtectionContext{Status: "unavailable", ExecutionCount: len(ids)}
	var first ProtectionRecord
	same := true
	for _, id := range ids {
		record, ok := records[id]
		if !ok {
			continue
		}
		out.MatchedExecutions++
		if first.Bucket == "" {
			first = record
		} else if first.Bucket != record.Bucket || first.Identity != record.Identity {
			same = false
		}
		if record.At.After(out.EvidenceAt) {
			out.EvidenceAt = record.At
		}
	}
	if out.MatchedExecutions > 0 {
		out.Status = "partial"
	}
	if len(ids) > 0 && out.MatchedExecutions == len(ids) && same {
		out.Status = "linked"
		out.Bucket = first.Bucket
	}
	return out
}
