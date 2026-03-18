//go:build darwin && cgo

package onboarding

/*
#include <stdlib.h>
// Only extern declarations — definitions live in wizard_darwin_impl.go.
// CGo generates _cgo_export.h which provides these at link time to that file.
extern void wizard_eval_js(const char *js);
extern void wizard_close(void);
*/
import "C"

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/kardianos/service"
	"github.com/ms/agent-daemon/internal/config"
	"github.com/ms/agent-daemon/internal/platform"
	internalsvc "github.com/ms/agent-daemon/internal/service"
	"github.com/ms/agent-daemon/internal/store"
)

// wizardGoMessage is called from ObjC when JS posts to window.webkit.messageHandlers.agent.
// Messages: setAnthropicKey, setOpenAIKey, openSettings, done.
//
//export wizardGoMessage
func wizardGoMessage(cAction *C.char, cPayload *C.char) {
	action := C.GoString(cAction)
	payload := C.GoString(cPayload)
	if gState == nil {
		return
	}
	switch action {
	case "setAnthropicKey":
		gState.anthropicKey = payload
	case "setOpenAIKey":
		gState.openAIKey = payload
	case "openSettings":
		openSystemSettings()
		go pollFDA(gState)
	case "done":
		go handleDone(gState)
	}
}

// wizardGoActivation is called from NSNotificationCenter when the app becomes active.
// Primary FDA detection signal: user returned from System Settings.
//
//export wizardGoActivation
func wizardGoActivation() {
	if gState == nil || gState.fdaGranted.Load() {
		return
	}
	if CheckFDA() {
		gState.fdaGranted.Store(true)
		pushJS(`window.dispatchEvent(new CustomEvent('fdaGranted'))`)
	}
}

// handleDone runs the Done-button flow:
//  1. Save API keys to BoltDB
//  2. Capture UserContext (HomeDir, Shell, UID) into BoltDB
//  3. Install the service (user-level LaunchAgent) if not already installed
//  4. Start the service
//  5. Mark OnboardingComplete = true
//  6. Close the wizard and notify the tray
func handleDone(s *state) {
	st, err := store.Open(platform.DBPath())
	if err != nil {
		pushInstallError("Failed to open database: " + err.Error())
		return
	}
	defer st.Close()

	cfg, err := st.GetConfig(context.Background())
	if err != nil {
		pushInstallError("Failed to read config: " + err.Error())
		return
	}

	// Save API keys
	cfg.AnthropicKey = s.anthropicKey
	cfg.OpenAIKey = s.openAIKey

	// Capture user context (HomeDir, Shell, UID)
	if uc := config.CaptureUserContext(); uc != nil {
		cfg.UserContext = uc
		slog.Info("onboarding: captured user context", "home", uc.HomeDir, "shell", uc.Shell)
	}

	if err := st.SaveConfig(context.Background(), cfg); err != nil {
		pushInstallError("Failed to save config: " + err.Error())
		return
	}

	// Install service only if not already installed (kardianos/service is not idempotent)
	if !isServiceInstalled() {
		svc, err := internalsvc.NewServiceForControl(internalsvc.LevelUser)
		if err != nil {
			pushInstallError("Failed to create service config: " + err.Error())
			return
		}
		if err := service.Control(svc, "install"); err != nil {
			pushInstallError("Service install failed: " + err.Error())
			return
		}
		slog.Info("onboarding: service installed")
	}

	// Start service (best-effort; may already be running)
	if svc, err := internalsvc.NewServiceForControl(internalsvc.LevelUser); err == nil {
		_ = service.Control(svc, "start")
	}

	// Mark onboarding complete
	cfg.OnboardingComplete = true
	if err := st.SaveConfig(context.Background(), cfg); err != nil {
		slog.Warn("onboarding: failed to save OnboardingComplete", "err", err)
	}

	// Close the wizard
	s.closed.Store(true)
	gState = nil
	cEmpty := C.CString("")
	defer C.free(unsafe.Pointer(cEmpty))
	C.wizard_close()

	if s.onDone != nil {
		s.onDone()
	}
}

// openSystemSettings deep-links to Privacy & Security → Full Disk Access.
func openSystemSettings() {
	if err := exec.Command("open",
		"x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles",
	).Start(); err != nil {
		slog.Warn("onboarding: failed to open System Settings", "err", err)
	}
}

// isServiceInstalled checks whether the LaunchAgent or LaunchDaemon plist exists.
func isServiceInstalled() bool {
	home, _ := os.UserHomeDir()
	if _, err := os.Stat(filepath.Join(home, "Library", "LaunchAgents", "agent-daemon.plist")); err == nil {
		return true
	}
	if _, err := os.Stat("/Library/LaunchDaemons/agent-daemon.plist"); err == nil {
		return true
	}
	return false
}

// pushInstallError sends an installError event to the wizard JS layer.
func pushInstallError(msg string) {
	safe := strings.ReplaceAll(msg, `\`, `\\`)
	safe = strings.ReplaceAll(safe, `"`, `\"`)
	pushJS(fmt.Sprintf(`window.dispatchEvent(new CustomEvent('installError', {detail: {msg: "%s"}}))`, safe))
}
