package store

import "testing"

func TestTelemetryIdentityRoundTrip(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, ok, err := s.GetTelemetryIdentity(); err != nil {
		t.Fatalf("GetTelemetryIdentity: %v", err)
	} else if ok {
		t.Fatal("expected no identity before registration")
	}

	if err := s.SetTelemetryIdentity("instance-1", "token-1"); err != nil {
		t.Fatalf("SetTelemetryIdentity: %v", err)
	}

	got, ok, err := s.GetTelemetryIdentity()
	if err != nil {
		t.Fatalf("GetTelemetryIdentity: %v", err)
	}
	if !ok {
		t.Fatal("expected identity after registration")
	}
	if got.InstanceID != "instance-1" || got.Token != "token-1" {
		t.Fatalf("identity = %+v, want instance-1/token-1", got)
	}
}

func TestTelemetryEnabledDefaultsFalse(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	enabled, err := s.TelemetryEnabled()
	if err != nil {
		t.Fatalf("TelemetryEnabled: %v", err)
	}
	if enabled {
		t.Fatal("expected telemetry disabled by default")
	}
}

func TestTelemetryEnabledToggleSurvivesReopen(t *testing.T) {
	dir := t.TempDir()

	s1, err := Open(dir)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := s1.SetTelemetryEnabled(true); err != nil {
		t.Fatalf("SetTelemetryEnabled(true): %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	if enabled, err := s2.TelemetryEnabled(); err != nil {
		t.Fatalf("TelemetryEnabled: %v", err)
	} else if !enabled {
		t.Fatal("expected telemetry to stay enabled after reopen")
	}

	if err := s2.SetTelemetryEnabled(false); err != nil {
		t.Fatalf("SetTelemetryEnabled(false): %v", err)
	}
	if enabled, err := s2.TelemetryEnabled(); err != nil {
		t.Fatalf("TelemetryEnabled: %v", err)
	} else if enabled {
		t.Fatal("expected telemetry disabled after toggling off")
	}
}
