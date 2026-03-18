package config

import (
	"testing"
)

func TestCaptureUserContext_ReturnsValidContext(t *testing.T) {
	uc := CaptureUserContext()
	if uc == nil {
		t.Fatal("expected non-nil UserContext")
	}
	if uc.HomeDir == "" {
		t.Error("HomeDir should not be empty")
	}
	if uc.Username == "" {
		t.Error("Username should not be empty")
	}
	if uc.Shell == "" {
		t.Error("Shell should not be empty")
	}
}

func TestConfigHasOnboardingComplete(t *testing.T) {
	cfg := Defaults()
	if cfg.OnboardingComplete {
		t.Error("OnboardingComplete should default to false")
	}
	cfg.OnboardingComplete = true
	if !cfg.OnboardingComplete {
		t.Error("OnboardingComplete should be settable to true")
	}
}
