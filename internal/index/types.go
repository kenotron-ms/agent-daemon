package index

// Capability describes a single capability (bundle, agent, recipe, behavior, etc.)
// discovered in a repository.
type Capability struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`
	SourceFile  string `json:"sourceFile"`
}

// Entry is the index record for one GitHub repository.
type Entry struct {
	Remote        string       `json:"remote"`        // "org/repo"
	Name          string       `json:"name"`
	Description   string       `json:"description,omitempty"`
	DefaultBranch string       `json:"defaultBranch"`
	Stars         int          `json:"stars"`
	Private       bool         `json:"private"`
	Topics        []string     `json:"topics"`
	Install       string       `json:"install"`
	Capabilities  []Capability `json:"capabilities"`
	ScannedAt     string       `json:"scannedAt"`
}

// IndexFile is the on-disk format for index.json.
type IndexFile struct {
	Version  int              `json:"version"`
	LastScan string           `json:"lastScan"`
	Repos    map[string]Entry `json:"repos"`
}

// RepoState tracks per-repo scan state for incremental updates.
type RepoState struct {
	PushedAt      string `json:"pushedAt"`
	TreeSha       string `json:"treeSha"`
	AmplifierLike bool   `json:"amplifierLike"`
}

// StateFile is the on-disk format for state.json.
type StateFile struct {
	Version   int                  `json:"version"`
	Repos     map[string]RepoState `json:"repos"`
	RateLimit *RateLimitInfo       `json:"rateLimit,omitempty"`
}

// RateLimitInfo holds the last known GitHub rate-limit state.
type RateLimitInfo struct {
	Remaining int   `json:"remaining"`
	ResetAt   int64 `json:"resetAt"`
}
