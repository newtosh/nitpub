// Package installcheck validates a nitpub instance configuration and runtime health.
package installcheck

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/newtosh/nitpub/internal/config"
)

// Severity classifies a check result.
type Severity int

const (
	SeverityOK Severity = iota
	SeverityWarn
	SeverityFail
)

// Result is one doctor check outcome.
type Result struct {
	Name     string
	Severity Severity
	Message  string
}

// Report aggregates doctor results.
type Report struct {
	Results []Result
}

func (r *Report) add(name string, sev Severity, msg string) {
	r.Results = append(r.Results, Result{Name: name, Severity: sev, Message: msg})
}

// Failed reports whether any hard-fail checks are present.
func (r Report) Failed() bool {
	for _, res := range r.Results {
		if res.Severity == SeverityFail {
			return true
		}
	}
	return false
}

// RunConfigChecks validates loaded config core keys and optional analytics completeness.
func RunConfigChecks(cfg *config.Config) Report {
	var r Report
	if cfg == nil {
		r.add("config", SeverityFail, "no config loaded")
		return r
	}
	if strings.TrimSpace(cfg.Domain) == "" {
		r.add("domain", SeverityFail, "domain is required")
	} else {
		r.add("domain", SeverityOK, cfg.Domain)
	}
	if strings.TrimSpace(cfg.Actor) == "" {
		r.add("actor", SeverityFail, "actor is required")
	} else {
		r.add("actor", SeverityOK, cfg.Actor)
	}
	if strings.TrimSpace(cfg.Secret) == "" || cfg.Secret == "CHANGE-ME" || cfg.Secret == "dev-secret-change-me" {
		r.add("secret", SeverityFail, "secret must be set to a non-default value")
	} else {
		r.add("secret", SeverityOK, "set")
	}
	if cfg.AnalyticsEnabled {
		if strings.TrimSpace(cfg.AnalyticsAPIToken) == "" || cfg.AnalyticsAPIToken == "CHANGE-ME" {
			r.add("analytics", SeverityWarn, "analytics_enabled but analytics_api_token is missing")
		} else if strings.TrimSpace(cfg.AnalyticsBaseURL) == "" {
			r.add("analytics", SeverityWarn, "analytics_enabled but analytics_base_url is empty")
		} else {
			r.add("analytics", SeverityOK, "enabled")
		}
	} else {
		r.add("analytics", SeverityOK, "disabled")
	}
	return r
}

// CheckSystemdActive best-effort checks whether a systemd unit is active.
// Missing systemctl or inactive unit yields a warning (not always a hard fail —
// install may run before enable).
func CheckSystemdActive(unit string) Result {
	if unit == "" {
		unit = "nitpub"
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return Result{Name: "systemd", Severity: SeverityWarn, Message: "systemctl not found"}
	}
	out, err := exec.Command("systemctl", "is-active", unit).CombinedOutput()
	state := strings.TrimSpace(string(out))
	if err != nil || state != "active" {
		return Result{Name: "systemd", Severity: SeverityWarn, Message: fmt.Sprintf("%s is %q (expected active)", unit, state)}
	}
	return Result{Name: "systemd", Severity: SeverityOK, Message: unit + " active"}
}

// CheckBinary reports whether path exists and is executable.
func CheckBinary(path string) Result {
	if path == "" {
		path = "/usr/local/bin/nitpub"
	}
	st, err := os.Stat(path)
	if err != nil {
		return Result{Name: "binary", Severity: SeverityFail, Message: path + " not found"}
	}
	if st.IsDir() {
		return Result{Name: "binary", Severity: SeverityFail, Message: path + " is a directory"}
	}
	if st.Mode()&0o111 == 0 {
		return Result{Name: "binary", Severity: SeverityFail, Message: path + " is not executable"}
	}
	return Result{Name: "binary", Severity: SeverityOK, Message: path}
}
