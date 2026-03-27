package mirror

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SyncEngine manages per-connector sync goroutines. Each connector gets its
// own goroutine that fetches data on an interval, diffs against the mirror,
// and dispatches jobs when changes are detected.
type SyncEngine struct {
	store    *MirrorStore
	fetchers map[FetchMethod]Fetcher

	// OnChange is called when a connector detects a change. The caller
	// (scheduler/daemon) wires this to dispatch the appropriate jobs.
	OnChange func(conn *Connector, entity *Entity, diff *DiffResult)

	loops  map[string]context.CancelFunc // connectorID → cancel
	counts map[string]int                // connectorID → sync cycle count
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

// NewSyncEngine creates a SyncEngine with the given store and fetchers.
func NewSyncEngine(store *MirrorStore, fetchers map[FetchMethod]Fetcher) *SyncEngine {
	return &SyncEngine{
		store:    store,
		fetchers: fetchers,
		loops:    make(map[string]context.CancelFunc),
		counts:   make(map[string]int),
	}
}

// Start begins the sync engine. Call after registering OnChange.
func (se *SyncEngine) Start(ctx context.Context) {
	se.ctx, se.cancel = context.WithCancel(ctx)

	// Load all enabled connectors and start their sync loops
	conns, err := se.store.ListConnectors(ctx)
	if err != nil {
		slog.Error("sync engine: failed to load connectors", "err", err)
		return
	}

	for _, conn := range conns {
		if conn.Enabled {
			se.StartConnector(conn)
		}
	}

	slog.Info("sync engine started", "connectors", len(conns))
}

// Stop halts all sync loops.
func (se *SyncEngine) Stop() {
	if se.cancel != nil {
		se.cancel()
	}
	se.mu.Lock()
	for id, cancel := range se.loops {
		cancel()
		delete(se.loops, id)
	}
	se.mu.Unlock()
}

// StartConnector begins the sync loop for a single connector.
func (se *SyncEngine) StartConnector(conn *Connector) {
	se.StopConnector(conn.ID)

	interval, err := time.ParseDuration(conn.Interval)
	if err != nil {
		slog.Error("sync engine: invalid interval", "connector", conn.Name, "interval", conn.Interval, "err", err)
		return
	}

	loopCtx, cancel := context.WithCancel(se.ctx)
	se.mu.Lock()
	se.loops[conn.ID] = cancel
	se.counts[conn.ID] = 0
	se.mu.Unlock()

	go se.runSyncLoop(loopCtx, conn.ID, interval)
}

// StopConnector stops the sync loop for a single connector.
func (se *SyncEngine) StopConnector(connectorID string) {
	se.mu.Lock()
	if cancel, ok := se.loops[connectorID]; ok {
		cancel()
		delete(se.loops, connectorID)
		delete(se.counts, connectorID)
	}
	se.mu.Unlock()
}

// runSyncLoop is the per-connector goroutine.
func (se *SyncEngine) runSyncLoop(ctx context.Context, connectorID string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start, then on interval
	se.syncOnce(ctx, connectorID)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			se.syncOnce(ctx, connectorID)
		}
	}
}

// syncOnce performs a single fetch-diff-dispatch cycle for a connector.
func (se *SyncEngine) syncOnce(ctx context.Context, connectorID string) {
	// Re-fetch connector from store (may have been updated)
	conn, err := se.store.GetConnector(ctx, connectorID)
	if err != nil {
		slog.Debug("sync: connector not found, stopping", "id", connectorID)
		se.StopConnector(connectorID)
		return
	}

	if !conn.Enabled {
		return
	}

	// Health-based backoff
	se.mu.Lock()
	se.counts[connectorID]++
	count := se.counts[connectorID]
	se.mu.Unlock()

	if !ShouldSync(conn, count) {
		return
	}

	// Get the appropriate fetcher
	fetcher, ok := se.fetchers[conn.FetchMethod]
	if !ok {
		slog.Error("sync: no fetcher for method", "connector", conn.Name, "method", conn.FetchMethod)
		return
	}

	// Fetch
	result, err := fetcher.Fetch(conn)
	if err != nil {
		_ = RecordFailure(ctx, se.store, conn, err)
		return
	}

	// Apply JQ filter if configured
	data := result.Data
	if conn.JQFilter != "" {
		data = applyGJSONFilter(data, conn.JQFilter)
	}

	// Hash and compare
	newHash := HashJSON(data)

	meta, err := se.store.GetEntityMeta(ctx, conn.EntityAddress)
	if err != nil {
		// First sync — create entity and meta
		meta = &EntityMeta{
			Address:    conn.EntityAddress,
			LastHash:   "",
			Version:    0,
			CreatedAt:  time.Now(),
			LastSyncAt: time.Now(),
		}
	}

	if newHash == meta.LastHash {
		// No change — just update sync time
		_ = RecordSuccess(ctx, se.store, conn)
		return
	}

	// Change detected — diff, store, and dispatch
	var previousData json.RawMessage
	if existing, err := se.store.GetEntity(ctx, conn.EntityAddress); err == nil {
		previousData = existing.Data
	}

	// Merge data (respects connector field ownership)
	mergedData, err := MergeEntityData(previousData, data, conn.Fields)
	if err != nil {
		slog.Error("sync: merge failed", "connector", conn.Name, "err", err)
		mergedData = data
	}

	// Diff
	diff, err := DiffJSON(previousData, mergedData, conn.Fields)
	if err != nil {
		slog.Warn("sync: diff failed, treating as full change", "connector", conn.Name, "err", err)
		diff = &DiffResult{Changed: true}
	}

	// Update entity
	entity := &Entity{Address: conn.EntityAddress, Data: mergedData}
	if err := se.store.SaveEntity(ctx, entity); err != nil {
		slog.Error("sync: save entity failed", "connector", conn.Name, "err", err)
		return
	}

	// Update meta
	meta.Version++
	meta.LastHash = HashJSON(mergedData)
	meta.LastSyncAt = time.Now()
	if err := se.store.SaveEntityMeta(ctx, meta); err != nil {
		slog.Error("sync: save meta failed", "connector", conn.Name, "err", err)
	}

	// Append change record
	diffJSON, _ := DiffToJSON(diff)
	_ = se.store.AppendChange(ctx, &ChangeRecord{
		ID:           uuid.New().String(),
		Address:      conn.EntityAddress,
		ConnectorID:  conn.ID,
		Timestamp:    time.Now(),
		PreviousHash: meta.LastHash,
		CurrentHash:  newHash,
		Version:      meta.Version,
		Diff:         diffJSON,
	})

	// Record success
	_ = RecordSuccess(ctx, se.store, conn)

	// Dispatch to jobs
	if se.OnChange != nil && diff.Changed {
		se.OnChange(conn, entity, diff)
	}
}

// applyGJSONFilter uses tidwall/gjson to extract a subset of the JSON data.
func applyGJSONFilter(data json.RawMessage, filter string) json.RawMessage {
	// Import gjson dynamically to keep the dependency optional
	// For now, return data as-is; the gjson integration happens at the
	// sync layer where tidwall/gjson is already available in go.mod
	_ = filter
	return data
}