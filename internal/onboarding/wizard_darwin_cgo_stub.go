//go:build darwin && cgo

package onboarding

// Temporary build stubs for darwin+cgo — replaced by wizard_darwin_impl.go (Task 6)
// and wizard_darwin_callbacks.go (Task 7) when the CGo bindings land.
// Required so the package compiles on macOS with CGO_ENABLED=1 (the default)
// before the Cocoa/WKWebView implementation is added.

// CheckFDA reports whether Full Disk Access has been granted.
// Stub returns false until the real probe is implemented in wizard_darwin_impl.go.
func CheckFDA() bool { return false }

// showImpl is the platform entry point called by Show().
// Stub is a no-op until the NSPanel+WKWebView implementation lands.
func showImpl(_ *state) {}
