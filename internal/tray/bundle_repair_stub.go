//go:build !darwin || !cgo

package tray

// repairBundle is a no-op on non-macOS platforms.
func repairBundle() {}
