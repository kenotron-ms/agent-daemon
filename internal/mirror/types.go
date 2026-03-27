package mirror

import (
	"encoding/json"
	"time"
)

// ── Connector ────────────────────────────────────────────────────────────────

// FetchMethod describes how a connector retrieves data from the outside world.
type FetchMethod string

const (
	FetchCommand FetchMethod = "command" // shell command whose stdout is the data
	FetchHTTP    FetchMethod = "http"    // GET/POST to a URL
	FetchBrowser FetchMethod = "browser" // headless browser via agent-browser
)

// ConnectorHealth tracks the operational status of a connector.
type ConnectorHealth string

const (
	HealthHealthy  ConnectorHealth = "healthy"
	HealthDegraded ConnectorHealth = "degraded"  // 1–2 consecutive failures
	HealthUnhealthy ConnectorHealth = "unhealthy" // 3+ consecutive failures
)

// Connector defines a data source to watch. The Prompt describes in natural
// language what to extract; the browser-operator agent interprets it each sync.
type Connector struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`

	// What to watch — the prompt IS the connector spec.
	Prompt string `json:"prompt"`
	URL    string `json:"url"`
	Site   string `json:"site,omitempty"` // groups browser profiles, e.g. "amazon", "github"

	// How to fetch
	FetchMethod FetchMethod `json:"fetchMethod"`
	Command     string      `json:"command,omitempty"`     // FetchCommand: shell command
	Headers     map[string]string `json:"headers,omitempty"` // FetchHTTP: request headers
	HTTPMethod  string      `json:"httpMethod,omitempty"`  // FetchHTTP: GET (default) or POST
	HTTPBody    string      `json:"httpBody,omitempty"`    // FetchHTTP: request body
	JQFilter    string      `json:"jqFilter,omitempty"`    // optional: gjson path to extract before diffing

	// What entity this connector writes to
	EntityAddress string   `json:"entityAddress"` // e.g. "github.pr/owner/repo/42"
	Fields        []string `json:"fields,omitempty"` // which top-level fields this connector owns

	// When and how often
	Interval string `json:"interval"` // Go duration, e.g. "60s", "5m"

	// Which jobs to fire on detected change
	JobIDs []string `json:"jobIds,omitempty"`

	Enabled   bool      `json:"enabled"`
	Health    ConnectorHealth `json:"health"`
	FailCount int       `json:"failCount"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	LastSyncAt *time.Time `json:"lastSyncAt,omitempty"`
}

// ── Entity ───────────────────────────────────────────────────────────────────

// Entity is the shadow copy — the digital twin of a slice of the external world.
// Address is the primary key: "{kind}/{identity}", e.g. "github.pr/owner/repo/42".
type Entity struct {
	Address string          `json:"address"`
	Data    json.RawMessage `json:"data"` // the actual snapshot, heterogeneous JSON
}

// EntityMeta holds bookkeeping for an entity, stored in a separate bucket
// so the hot path (fetch → hash compare) doesn't need to deserialize the full data.
type EntityMeta struct {
	Address    string    `json:"address"`
	LastHash   string    `json:"lastHash"`   // SHA256 of Data
	Version    int       `json:"version"`    // incremented on each real change
	LastSyncAt time.Time `json:"lastSyncAt"`
	CreatedAt  time.Time `json:"createdAt"`
}

// ── Change Record ────────────────────────────────────────────────────────────

// ChangeRecord is an immutable log entry of a detected change.
type ChangeRecord struct {
	ID           string          `json:"id"`
	Address      string          `json:"address"`      // entity address
	ConnectorID  string          `json:"connectorId"`  // which connector detected the change
	Timestamp    time.Time       `json:"timestamp"`
	PreviousHash string          `json:"previousHash"`
	CurrentHash  string          `json:"currentHash"`
	Version      int             `json:"version"`      // entity version after this change
	Diff         json.RawMessage `json:"diff"`         // structured diff (added/removed/modified)
}

// ── Diff Types ───────────────────────────────────────────────────────────────

// DiffResult describes what changed between two snapshots of an entity.
type DiffResult struct {
	Changed bool        `json:"changed"`
	Added   []FieldDiff `json:"added,omitempty"`
	Removed []FieldDiff `json:"removed,omitempty"`
	Modified []FieldDiff `json:"modified,omitempty"`
}

// FieldDiff describes a single field-level change.
type FieldDiff struct {
	Path     string          `json:"path"`               // JSON path, e.g. "comments.2.body"
	OldValue json.RawMessage `json:"oldValue,omitempty"`
	NewValue json.RawMessage `json:"newValue,omitempty"`
}

// ── Fetch Result ─────────────────────────────────────────────────────────────

// FetchResult is the output of a fetcher — the raw data extracted from the source.
type FetchResult struct {
	Data      json.RawMessage `json:"data"`
	Error     error           `json:"-"`
	FetchedAt time.Time       `json:"fetchedAt"`
}

// ── Fetcher Interface ────────────────────────────────────────────────────────

// Fetcher retrieves data from an external source. Implementations exist for
// command (shell), HTTP, and browser (agent-browser) fetch methods.
type Fetcher interface {
	Fetch(conn *Connector) (*FetchResult, error)
}