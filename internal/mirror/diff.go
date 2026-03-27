package mirror

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// HashJSON returns the SHA256 hex digest of the canonical JSON representation.
// Used for fast equality checks between snapshots.
func HashJSON(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// DiffJSON compares two JSON snapshots and produces a structured DiffResult.
// It operates on top-level keys only for simplicity and performance — nested
// changes appear as the full old/new value of the containing key.
//
// If fields is non-empty, only those top-level keys are compared (connector
// field ownership). Otherwise, all keys are compared.
func DiffJSON(previous, current json.RawMessage, fields []string) (*DiffResult, error) {
	if len(previous) == 0 && len(current) == 0 {
		return &DiffResult{Changed: false}, nil
	}

	prevMap, err := toMap(previous)
	if err != nil {
		// If previous wasn't a JSON object, treat as full replacement
		return diffNonObject(previous, current)
	}
	currMap, err := toMap(current)
	if err != nil {
		return diffNonObject(previous, current)
	}

	// If fields are specified, filter to only those keys
	if len(fields) > 0 {
		fieldSet := make(map[string]bool, len(fields))
		for _, f := range fields {
			fieldSet[f] = true
		}
		prevMap = filterMap(prevMap, fieldSet)
		currMap = filterMap(currMap, fieldSet)
	}

	result := &DiffResult{}

	// Find added and modified keys
	for key, currVal := range currMap {
		prevVal, existed := prevMap[key]
		if !existed {
			result.Added = append(result.Added, FieldDiff{
				Path:     key,
				NewValue: currVal,
			})
			continue
		}
		if !jsonEqual(prevVal, currVal) {
			result.Modified = append(result.Modified, FieldDiff{
				Path:     key,
				OldValue: prevVal,
				NewValue: currVal,
			})
		}
	}

	// Find removed keys
	for key, prevVal := range prevMap {
		if _, exists := currMap[key]; !exists {
			result.Removed = append(result.Removed, FieldDiff{
				Path:     key,
				OldValue: prevVal,
			})
		}
	}

	// Sort for deterministic output
	sortDiffs(result.Added)
	sortDiffs(result.Removed)
	sortDiffs(result.Modified)

	result.Changed = len(result.Added) > 0 || len(result.Removed) > 0 || len(result.Modified) > 0
	return result, nil
}

// DiffToJSON marshals a DiffResult to JSON for storage in ChangeRecord.
func DiffToJSON(d *DiffResult) (json.RawMessage, error) {
	data, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// MergeEntityData performs a shallow merge of new data into existing entity data.
// If fields is non-empty, only those top-level keys from newData are merged.
// If fields is empty, newData replaces existing entirely.
func MergeEntityData(existing, newData json.RawMessage, fields []string) (json.RawMessage, error) {
	if len(fields) == 0 {
		// Full replacement
		return newData, nil
	}

	existingMap, err := toMap(existing)
	if err != nil {
		existingMap = make(map[string]json.RawMessage)
	}
	newMap, err := toMap(newData)
	if err != nil {
		return newData, nil
	}

	// Only merge the owned fields
	fieldSet := make(map[string]bool, len(fields))
	for _, f := range fields {
		fieldSet[f] = true
	}

	for key, val := range newMap {
		if fieldSet[key] {
			existingMap[key] = val
		}
	}

	return json.Marshal(existingMap)
}

// ── internal helpers ─────────────────────────────────────────────────────────

func toMap(data json.RawMessage) (map[string]json.RawMessage, error) {
	if len(data) == 0 {
		return make(map[string]json.RawMessage), nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = make(map[string]json.RawMessage)
	}
	return m, nil
}

func filterMap(m map[string]json.RawMessage, fields map[string]bool) map[string]json.RawMessage {
	filtered := make(map[string]json.RawMessage, len(fields))
	for k, v := range m {
		if fields[k] {
			filtered[k] = v
		}
	}
	return filtered
}

// jsonEqual compares two JSON values for semantic equality (ignores key order).
func jsonEqual(a, b json.RawMessage) bool {
	// Fast path: byte equality
	if string(a) == string(b) {
		return true
	}
	// Slow path: normalize and compare
	var va, vb any
	if err := json.Unmarshal(a, &va); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return false
	}
	na, _ := json.Marshal(va)
	nb, _ := json.Marshal(vb)
	return string(na) == string(nb)
}

// diffNonObject handles the case where either value is not a JSON object.
func diffNonObject(previous, current json.RawMessage) (*DiffResult, error) {
	changed := !jsonEqual(previous, current)
	result := &DiffResult{Changed: changed}
	if changed {
		result.Modified = []FieldDiff{{
			Path:     ".",
			OldValue: previous,
			NewValue: current,
		}}
	}
	return result, nil
}

func sortDiffs(diffs []FieldDiff) {
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Path < diffs[j].Path
	})
}

// ensure fmt is used (for potential future debug logging)
var _ = fmt.Sprintf