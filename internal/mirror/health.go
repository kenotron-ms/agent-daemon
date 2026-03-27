package mirror

import (
	"context"
	"log/slog"
	"time"
)

const (
	// HealthDegradedThreshold is the number of consecutive failures before
	// a connector is marked degraded.
	HealthDegradedThreshold = 1
	// HealthUnhealthyThreshold is the number of consecutive failures before
	// a connector is marked unhealthy.
	HealthUnhealthyThreshold = 3
	// DefaultPruneAge is the default age for pruning change records.
	DefaultPruneAge = 7 * 24 * time.Hour // 7 days
)

// RecordSuccess resets a connector's fail count and marks it healthy.
func RecordSuccess(ctx context.Context, store *MirrorStore, conn *Connector) error {
	conn.FailCount = 0
	conn.Health = HealthHealthy
	now := time.Now()
	conn.LastSyncAt = &now
	conn.UpdatedAt = now
	return store.SaveConnector(ctx, conn)
}

// RecordFailure increments a connector's fail count and updates its health status.
func RecordFailure(ctx context.Context, store *MirrorStore, conn *Connector, fetchErr error) error {
	conn.FailCount++
	switch {
	case conn.FailCount >= HealthUnhealthyThreshold:
		conn.Health = HealthUnhealthy
	case conn.FailCount >= HealthDegradedThreshold:
		conn.Health = HealthDegraded
	}
	conn.UpdatedAt = time.Now()

	slog.Warn("connector fetch failed",
		"connector", conn.Name,
		"id", conn.ID,
		"failCount", conn.FailCount,
		"health", conn.Health,
		"error", fetchErr,
	)

	return store.SaveConnector(ctx, conn)
}

// PruneOldChanges removes change records older than the given age.
// Returns the number of records pruned.
func PruneOldChanges(ctx context.Context, store *MirrorStore, age time.Duration) (int, error) {
	if age == 0 {
		age = DefaultPruneAge
	}
	pruned, err := store.PruneChanges(ctx, age)
	if err != nil {
		return 0, err
	}
	if pruned > 0 {
		slog.Info("pruned old change records", "count", pruned, "age", age)
	}
	return pruned, nil
}

// IsHealthy returns true if the connector is in a healthy state.
func IsHealthy(conn *Connector) bool {
	return conn.Health == HealthHealthy || conn.Health == ""
}

// ShouldSync returns true if the connector should attempt a sync.
// Unhealthy connectors still sync but at a reduced rate (every 3rd attempt).
func ShouldSync(conn *Connector, syncCount int) bool {
	if !conn.Enabled {
		return false
	}
	if conn.Health == HealthUnhealthy {
		return syncCount%3 == 0 // back off: only try every 3rd cycle
	}
	return true
}