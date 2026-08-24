package installcheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/newtosh/nitpub/internal/config"
)

func TestRunConfigChecks_missingCore(t *testing.T) {
	r := RunConfigChecks(&config.Config{})
	if !r.Failed() {
		t.Fatal("expected fail for empty config")
	}
}

func TestRunConfigChecks_healthy(t *testing.T) {
	r := RunConfigChecks(&config.Config{
		Domain: "blog.example.com",
		Actor:  "alice",
		Secret: "a-real-secret-value-32chars-min!!",
	})
	if r.Failed() {
		t.Fatalf("unexpected fail: %+v", r.Results)
	}
}

func TestRunConfigChecks_analyticsWarn(t *testing.T) {
	r := RunConfigChecks(&config.Config{
		Domain:            "blog.example.com",
		Actor:             "alice",
		Secret:            "a-real-secret-value-32chars-min!!",
		AnalyticsEnabled:  true,
		AnalyticsAPIToken: "",
		AnalyticsBaseURL:  "http://127.0.0.1:8181",
	})
	if r.Failed() {
		t.Fatal("analytics gap should warn, not fail")
	}
	found := false
	for _, res := range r.Results {
		if res.Name == "analytics" && res.Severity == SeverityWarn {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected analytics warn, got %+v", r.Results)
	}
}

func TestCheckBinary(t *testing.T) {
	dir := t.TempDir()
	missing := CheckBinary(filepath.Join(dir, "nope"))
	if missing.Severity != SeverityFail {
		t.Fatalf("expected fail, got %+v", missing)
	}
	p := filepath.Join(dir, "nitpub")
	if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	ok := CheckBinary(p)
	if ok.Severity != SeverityOK {
		t.Fatalf("expected ok, got %+v", ok)
	}
	noExec := filepath.Join(dir, "locked")
	if err := os.WriteFile(noExec, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := CheckBinary(noExec)
	if bad.Severity != SeverityFail {
		t.Fatalf("expected fail for non-executable, got %+v", bad)
	}
}
