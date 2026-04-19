package index

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ScanOptions controls how Scan operates.
type ScanOptions struct {
	Limit           int  // 0 = unlimited
	Force           bool // re-scan even if pushed_at unchanged
	IncludeArchived bool
	Quiet           bool
}

// ScanResult summarises what changed during a Scan run.
type ScanResult struct {
	Added        []Entry
	Updated      []Entry
	Removed      []string // remote keys ("org/repo") removed from the index
	Unchanged    int
	Skipped      int
	APIRemaining int
}

// ── package-level compiled regexps ──────────────────────────────────────────

var (
	reBehaviors = regexp.MustCompile(`^behaviors/([^/]+)\.yaml$`)
	reAgents    = regexp.MustCompile(`^agents/([^/]+)\.md$`)
	reRecipes   = regexp.MustCompile(`^recipes/([^/]+)\.yaml$`)
	reAmpRecipes = regexp.MustCompile(`^\.amplifier/recipes/([^/]+)\.yaml$`)
	rePySection  = regexp.MustCompile(`^\[([^\]]+)\]`)
	rePyName     = regexp.MustCompile(`^name\s*=\s*"([^"]*)"`)
	rePyDesc     = regexp.MustCompile(`^description\s*=\s*"([^"]*)"`)
)

// ── token resolution ─────────────────────────────────────────────────────────

// resolveToken returns: GITHUB_TOKEN env var → gh auth token → empty string.
func resolveToken() string {
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		return tok
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err == nil {
		if tok := strings.TrimSpace(string(out)); tok != "" {
			return tok
		}
	}
	return ""
}

// ── GitHub API helper ────────────────────────────────────────────────────────

// ghGet makes a GET request to https://api.github.com/{path}.
// It sleeps *delay before the request, then updates *remaining/*resetAt from
// X-RateLimit-* response headers and adjusts *delay for the next call.
// Returns (nil, 304, nil) for 304 Not Modified.
func ghGet(ctx context.Context, token, path string, delay *time.Duration, remaining *int, resetAt *int64) ([]byte, int, error) {
	if *delay > 0 {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-time.After(*delay):
		}
	}

	url := "https://api.github.com/" + strings.TrimPrefix(path, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	// Track rate limits.
	if rem := resp.Header.Get("X-RateLimit-Remaining"); rem != "" {
		if v, e := strconv.Atoi(rem); e == nil {
			*remaining = v
		}
	}
	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		if v, e := strconv.ParseInt(reset, 10, 64); e == nil {
			*resetAt = v
		}
	}

	// Adjust delay for next call.
	if token != "" {
		switch {
		case *remaining < 20:
			now := time.Now().Unix()
			if *resetAt > now {
				*delay = time.Duration(*resetAt-now+5) * time.Second
			} else {
				*delay = 50 * time.Millisecond
			}
		case *remaining < 200:
			*delay = 500 * time.Millisecond
		default:
			*delay = 50 * time.Millisecond
		}
	} else {
		*delay = 1200 * time.Millisecond
	}

	if resp.StatusCode == http.StatusNotModified {
		return nil, 304, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// ── raw file fetch (no quota) ─────────────────────────────────────────────────

// rawFile fetches a raw file from raw.githubusercontent.com.
// Returns ("", nil) if the file is not found.
func rawFile(owner, repo, branch, path string) (string, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
		owner, repo, branch, path)
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ── GitHub API pagination ─────────────────────────────────────────────────────

// parseNextLink extracts the "next" URL from a GitHub Link header.
func parseNextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		segs := strings.SplitN(part, ";", 2)
		if len(segs) != 2 {
			continue
		}
		if strings.TrimSpace(segs[1]) == `rel="next"` {
			u := strings.TrimSpace(segs[0])
			return strings.Trim(u, "<>")
		}
	}
	return ""
}

// ListRepos returns all repos accessible to token via /user/repos, following
// Link header pagination.
func ListRepos(ctx context.Context, token string) ([]map[string]any, error) {
	var (
		all       []map[string]any
		delay     time.Duration
		remaining = 5000
		resetAt   int64
	)
	if token != "" {
		delay = 50 * time.Millisecond
	} else {
		delay = 1200 * time.Millisecond
	}

	nextURL := "https://api.github.com/user/repos" +
		"?per_page=100&sort=pushed&affiliation=owner,collaborator,organization_member"

	for nextURL != "" {
		if delay > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}

		// Track rate limits.
		if rem := resp.Header.Get("X-RateLimit-Remaining"); rem != "" {
			if v, e := strconv.Atoi(rem); e == nil {
				remaining = v
			}
		}
		if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
			if v, e := strconv.ParseInt(reset, 10, 64); e == nil {
				resetAt = v
			}
		}

		// Adjust delay for next call.
		if token != "" {
			switch {
			case remaining < 20:
				now := time.Now().Unix()
				if resetAt > now {
					delay = time.Duration(resetAt-now+5) * time.Second
				} else {
					delay = 50 * time.Millisecond
				}
			case remaining < 200:
				delay = 500 * time.Millisecond
			default:
				delay = 50 * time.Millisecond
			}
		} else {
			delay = 1200 * time.Millisecond
		}

		link := resp.Header.Get("Link")
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GitHub API returned %d listing repos", resp.StatusCode)
		}

		var page []map[string]any
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, err
		}
		all = append(all, page...)
		nextURL = parseNextLink(link)
	}

	return all, nil
}

// ── amplifier detection ───────────────────────────────────────────────────────

// treeIsAmplifierLike returns true when the path list contains amplifier
// signatures.
//
//   - Tier 1 (definitive): bundle.md, bundle.yaml, bundle.yml at root;
//     any path matching ^behaviors/[^/]+\.yaml$;
//     any path matching ^agents/[^/]+\.md$;
//     any path starting with .amplifier/
//   - Tier 2 (likely): any path matching ^recipes/[^/]+\.yaml$
func treeIsAmplifierLike(paths []string) bool {
	tier2 := false
	for _, p := range paths {
		switch p {
		case "bundle.md", "bundle.yaml", "bundle.yml":
			return true
		}
		if strings.HasPrefix(p, ".amplifier/") {
			return true
		}
		if reBehaviors.MatchString(p) || reAgents.MatchString(p) {
			return true
		}
		if reRecipes.MatchString(p) {
			tier2 = true
		}
	}
	return tier2
}

// ── capability extraction ─────────────────────────────────────────────────────

// parseFrontmatter extracts the YAML front matter from a markdown string.
// Returns nil if no front matter is found.
func parseFrontmatter(content string) map[string]any {
	// Normalise line endings.
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return nil
	}
	rest := content[4:] // skip "---\n"
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil
	}
	var out map[string]any
	_ = yaml.Unmarshal([]byte(rest[:end]), &out)
	return out
}

// nestedStr walks a nested map using the key path and returns the string value.
func nestedStr(data map[string]any, keys ...string) string {
	var cur any = data
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[k]
	}
	s, _ := cur.(string)
	return s
}

// readmeDescription extracts the first meaningful prose line from README text.
func readmeDescription(readme string) string {
	for _, line := range strings.Split(readme, "\n") {
		line = strings.TrimSpace(line)
		if line == "" ||
			strings.HasPrefix(line, "#") ||
			strings.HasPrefix(line, "![") ||
			strings.HasPrefix(line, "[![") ||
			strings.HasPrefix(line, "<!--") ||
			strings.HasPrefix(line, "---") {
			continue
		}
		runes := []rune(line)
		if len(runes) > 200 {
			runes = runes[:200]
		}
		return string(runes)
	}
	return ""
}

// parsePyprojectTOML extracts the project name and description from a
// pyproject.toml [project] section.
func parsePyprojectTOML(content string) (name, desc string) {
	inProject := false
	for _, line := range strings.Split(content, "\n") {
		if m := rePySection.FindStringSubmatch(line); m != nil {
			inProject = m[1] == "project"
			continue
		}
		if !inProject {
			continue
		}
		if name == "" {
			if m := rePyName.FindStringSubmatch(line); m != nil {
				name = m[1]
			}
		}
		if desc == "" {
			if m := rePyDesc.FindStringSubmatch(line); m != nil {
				desc = m[1]
			}
		}
		if name != "" && desc != "" {
			break
		}
	}
	return
}

// ExtractCapabilities reads specific files from the repository via
// raw.githubusercontent.com and returns detected capabilities.
//
// Passes:
//  1. bundle.md or bundle.yaml/yml — parse YAML/frontmatter, look for bundle.name
//  2. behaviors/*.yaml — parse YAML, look for bundle.name
//  3. agents/*.md — parse YAML frontmatter, look for meta.name
//  4. recipes/*.yaml and .amplifier/recipes/*.yaml — parse YAML, need name AND (steps or stages)
//  5. pyproject.toml — regex for [project] section, name= and description= fields
func ExtractCapabilities(owner, repo, branch string, paths []string, readmeText string) ([]Capability, error) {
	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}

	var caps []Capability
	var lastErr error

	// ── Pass 1: bundle definition file ──────────────────────────────────────
	for _, fname := range []string{"bundle.md", "bundle.yaml", "bundle.yml"} {
		if !pathSet[fname] {
			continue
		}
		content, err := rawFile(owner, repo, branch, fname)
		if err != nil {
			lastErr = err
			continue
		}
		if content == "" {
			continue
		}

		var data map[string]any
		if strings.HasSuffix(fname, ".md") {
			data = parseFrontmatter(content)
		} else {
			_ = yaml.Unmarshal([]byte(content), &data)
		}

		name := nestedStr(data, "bundle", "name")
		if name == "" {
			break
		}
		desc := nestedStr(data, "bundle", "description")
		if desc == "" {
			desc = readmeDescription(readmeText)
		}
		caps = append(caps, Capability{
			Type:        "bundle",
			Name:        name,
			Description: desc,
			Version:     nestedStr(data, "bundle", "version"),
			SourceFile:  fname,
		})
		break // only the first bundle file
	}

	// ── Pass 2: behaviors/*.yaml ─────────────────────────────────────────────
	for _, p := range paths {
		m := reBehaviors.FindStringSubmatch(p)
		if m == nil {
			continue
		}
		content, err := rawFile(owner, repo, branch, p)
		if err != nil || content == "" {
			continue
		}
		var data map[string]any
		_ = yaml.Unmarshal([]byte(content), &data)

		name := nestedStr(data, "bundle", "name")
		if name == "" {
			name = m[1] // fall back to file stem
		}
		caps = append(caps, Capability{
			Type:        "behavior",
			Name:        name,
			Description: nestedStr(data, "bundle", "description"),
			SourceFile:  p,
		})
	}

	// ── Pass 3: agents/*.md ───────────────────────────────────────────────────
	for _, p := range paths {
		m := reAgents.FindStringSubmatch(p)
		if m == nil {
			continue
		}
		content, err := rawFile(owner, repo, branch, p)
		if err != nil || content == "" {
			continue
		}
		data := parseFrontmatter(content)

		name := nestedStr(data, "meta", "name")
		if name == "" {
			name = m[1]
		}
		caps = append(caps, Capability{
			Type:        "agent",
			Name:        name,
			Description: nestedStr(data, "meta", "description"),
			SourceFile:  p,
		})
	}

	// ── Pass 4: recipes/*.yaml and .amplifier/recipes/*.yaml ─────────────────
	for _, p := range paths {
		var fallback string
		if m := reRecipes.FindStringSubmatch(p); m != nil {
			fallback = m[1]
		} else if m := reAmpRecipes.FindStringSubmatch(p); m != nil {
			fallback = m[1]
		} else {
			continue
		}

		content, err := rawFile(owner, repo, branch, p)
		if err != nil || content == "" {
			continue
		}
		var data map[string]any
		_ = yaml.Unmarshal([]byte(content), &data)

		name, _ := data["name"].(string)
		if name == "" {
			_ = fallback
			continue // name is required
		}
		_, hasSteps := data["steps"]
		_, hasStages := data["stages"]
		if !hasSteps && !hasStages {
			continue
		}
		caps = append(caps, Capability{
			Type:       "recipe",
			Name:       name,
			SourceFile: p,
		})
	}

	// ── Pass 5: pyproject.toml ────────────────────────────────────────────────
	if pathSet["pyproject.toml"] {
		content, err := rawFile(owner, repo, branch, "pyproject.toml")
		if err == nil && content != "" {
			if pName, pDesc := parsePyprojectTOML(content); pName != "" {
				caps = append(caps, Capability{
					Type:        "package",
					Name:        pName,
					Description: pDesc,
					SourceFile:  "pyproject.toml",
				})
			}
		}
	}

	return caps, lastErr
}

// ── main scan function ────────────────────────────────────────────────────────

// Scan scans GitHub repositories and updates the local index and state files.
func Scan(ctx context.Context, dir string, opts ScanOptions) (*ScanResult, error) {
	token := resolveToken()

	idx, err := LoadIndex(dir)
	if err != nil {
		return nil, fmt.Errorf("loading index: %w", err)
	}

	st, err := LoadState(dir)
	if err != nil {
		return nil, fmt.Errorf("loading state: %w", err)
	}

	if !opts.Quiet {
		fmt.Print("Fetching repo list from GitHub... ")
	}
	repos, err := ListRepos(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("listing repos: %w", err)
	}
	if !opts.Quiet {
		fmt.Printf("found %d repos\n", len(repos))
	}

	var (
		delay     time.Duration
		remaining = 5000
		resetAt   int64
	)
	if token != "" {
		delay = 50 * time.Millisecond
	} else {
		delay = 1200 * time.Millisecond
	}

	result := &ScanResult{}

	// Build a set of all repo keys for removal detection (key = "org/repo").
	repoKeys := make(map[string]bool, len(repos))
	for _, r := range repos {
		if k, ok := r["full_name"].(string); ok && k != "" {
			repoKeys[k] = true
		}
	}

	total := len(repos)
	limit := opts.Limit
	if limit <= 0 || limit > total {
		limit = total
	}

	if !opts.Quiet {
		fmt.Printf("Scanning %d repos...\n", limit)
	}

	for i, repo := range repos {
		if i >= limit {
			break
		}

		key, _ := repo["full_name"].(string)
		if key == "" {
			continue
		}

		// Skip archived repos unless requested.
		archived, _ := repo["archived"].(bool)
		if archived && !opts.IncludeArchived {
			result.Skipped++
			continue
		}

		pushedAt, _ := repo["pushed_at"].(string)
		defaultBranch, _ := repo["default_branch"].(string)
		if defaultBranch == "" {
			defaultBranch = "main"
		}

		cached := st.Repos[key]

		// ── Gate 1: pushed_at unchanged ────────────────────────────────────
		if pushedAt != "" && cached.PushedAt == pushedAt && !opts.Force {
			if cached.AmplifierLike {
				result.Unchanged++
			} else {
				result.Skipped++
			}
			if !opts.Quiet {
				fmt.Printf("  [%d/%d] %s (unchanged)\n", i+1, limit, key)
			}
			continue
		}

		// ── Fetch git tree ──────────────────────────────────────────────────
		treePath := fmt.Sprintf("repos/%s/git/trees/%s?recursive=1", key, defaultBranch)
		treeBody, status, err := ghGet(ctx, token, treePath, &delay, &remaining, &resetAt)
		if err != nil {
			if !opts.Quiet {
				fmt.Printf("  [%d/%d] %s (error: %v)\n", i+1, limit, key, err)
			}
			result.Skipped++
			continue
		}
		switch status {
		case 409: // empty repo
			st.Repos[key] = RepoState{PushedAt: pushedAt, AmplifierLike: false}
			result.Skipped++
			continue
		case 403, 404:
			result.Skipped++
			continue
		}
		if status != 200 {
			result.Skipped++
			continue
		}

		var treeResp struct {
			SHA  string `json:"sha"`
			Tree []struct {
				Path string `json:"path"`
			} `json:"tree"`
		}
		if err := json.Unmarshal(treeBody, &treeResp); err != nil {
			result.Skipped++
			continue
		}
		treeSha := treeResp.SHA
		paths := make([]string, 0, len(treeResp.Tree))
		for _, t := range treeResp.Tree {
			paths = append(paths, t.Path)
		}

		// ── Gate 2: tree SHA unchanged ─────────────────────────────────────
		if treeSha != "" && cached.TreeSha == treeSha && !opts.Force {
			st.Repos[key] = RepoState{
				PushedAt:      pushedAt,
				TreeSha:       cached.TreeSha,
				AmplifierLike: cached.AmplifierLike,
			}
			if cached.AmplifierLike {
				result.Unchanged++
			} else {
				result.Skipped++
			}
			if !opts.Quiet {
				fmt.Printf("  [%d/%d] %s (tree unchanged)\n", i+1, limit, key)
			}
			continue
		}

		// ── Amplifier-like check ───────────────────────────────────────────
		isAmplifier := treeIsAmplifierLike(paths)
		st.Repos[key] = RepoState{
			PushedAt:      pushedAt,
			TreeSha:       treeSha,
			AmplifierLike: isAmplifier,
		}

		if !isAmplifier {
			if _, exists := idx.Repos[key]; exists {
				result.Removed = append(result.Removed, key)
				delete(idx.Repos, key)
			} else {
				result.Skipped++
			}
			if !opts.Quiet {
				fmt.Printf("  [%d/%d] %s (not amplifier)\n", i+1, limit, key)
			}
			continue
		}

		// ── Fetch README for description fallback ──────────────────────────
		parts := strings.SplitN(key, "/", 2)
		ownerName, repoName := parts[0], parts[1]
		readmeText, _ := rawFile(ownerName, repoName, defaultBranch, "README.md")

		// ── Extract capabilities ────────────────────────────────────────────
		caps, _ := ExtractCapabilities(ownerName, repoName, defaultBranch, paths, readmeText)

		// ── Build entry ────────────────────────────────────────────────────
		name, _ := repo["name"].(string)
		desc, _ := repo["description"].(string)
		if desc == "" {
			desc = readmeDescription(readmeText)
		}
		stars, _ := repo["stargazers_count"].(float64)
		private, _ := repo["private"].(bool)
		topicsAny, _ := repo["topics"].([]any)
		topics := make([]string, 0, len(topicsAny))
		for _, t := range topicsAny {
			if s, ok := t.(string); ok {
				topics = append(topics, s)
			}
		}

		entry := Entry{
			Remote:        key,
			Name:          name,
			Description:   desc,
			DefaultBranch: defaultBranch,
			Stars:         int(stars),
			Private:       private,
			Topics:        topics,
			Install: fmt.Sprintf(
				"amplifier bundle add git+https://github.com/%s@%s",
				key, defaultBranch,
			),
			Capabilities: caps,
			ScannedAt:    time.Now().UTC().Format(time.RFC3339),
		}

		_, existed := idx.Repos[key]
		idx.Repos[key] = entry

		if existed {
			result.Updated = append(result.Updated, entry)
			if !opts.Quiet {
				fmt.Printf("  [%d/%d] %s → updated\n", i+1, limit, key)
			}
		} else {
			result.Added = append(result.Added, entry)
			if !opts.Quiet {
				fmt.Printf("  [%d/%d] %s → added\n", i+1, limit, key)
			}
		}
	}

	// ── Detect repos removed from GitHub ─────────────────────────────────────
	for k := range idx.Repos {
		if !repoKeys[k] {
			result.Removed = append(result.Removed, k)
			delete(idx.Repos, k)
		}
	}

	// ── Finalise and save ─────────────────────────────────────────────────────
	idx.LastScan = time.Now().UTC().Format(time.RFC3339)
	idx.Version = 1
	st.Version = 1
	st.RateLimit = &RateLimitInfo{Remaining: remaining, ResetAt: resetAt}
	result.APIRemaining = remaining

	if err := SaveIndex(dir, idx); err != nil {
		return nil, fmt.Errorf("saving index: %w", err)
	}
	if err := SaveState(dir, st); err != nil {
		return nil, fmt.Errorf("saving state: %w", err)
	}

	return result, nil
}
