package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/newtosh/nitpub/internal/cliui"
	"github.com/newtosh/nitpub/internal/config"
	"github.com/newtosh/nitpub/internal/installcheck"
)

func newDoctorCmd() *cobra.Command {
	var configPath string
	var binaryPath string
	var unit string
	var expectActive bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check nitpub install health",
		Long: `Validates core config keys, binary presence, and optional analytics completeness.
Warns (does not hard-fail) when systemd is inactive or analytics is partially configured.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(configPath, binaryPath, unit, expectActive)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "config.toml path (default: search order)")
	cmd.Flags().StringVar(&binaryPath, "binary", "/usr/local/bin/nitpub", "nitpub binary path to check")
	cmd.Flags().StringVar(&unit, "unit", "nitpub", "systemd unit name")
	cmd.Flags().BoolVar(&expectActive, "require-active", false, "hard-fail if systemd unit is not active")
	return cmd
}

func runDoctor(configPath, binaryPath, unit string, expectActive bool) error {
	cliui.Progress("running doctor checks")

	var report installcheck.Report
	if configPath != "" {
		prev := os.Getenv("NITPUB_CONFIG")
		_ = os.Setenv("NITPUB_CONFIG", configPath)
		defer func() {
			if prev == "" {
				_ = os.Unsetenv("NITPUB_CONFIG")
			} else {
				_ = os.Setenv("NITPUB_CONFIG", prev)
			}
		}()
	}
	cfg, err := config.Load()
	if err != nil {
		cliui.Fail("config: " + err.Error())
		report.Results = append(report.Results, installcheck.Result{
			Name: "config", Severity: installcheck.SeverityFail, Message: err.Error(),
		})
	} else {
		report = installcheck.RunConfigChecks(&cfg)
	}

	bin := installcheck.CheckBinary(binaryPath)
	report.Results = append(report.Results, bin)

	sys := installcheck.CheckSystemdActive(unit)
	if expectActive && sys.Severity != installcheck.SeverityOK {
		sys.Severity = installcheck.SeverityFail
	}
	report.Results = append(report.Results, sys)

	for _, res := range report.Results {
		switch res.Severity {
		case installcheck.SeverityOK:
			cliui.OK(fmt.Sprintf("%s: %s", res.Name, res.Message))
		case installcheck.SeverityWarn:
			cliui.Warn(fmt.Sprintf("%s: %s", res.Name, res.Message))
		case installcheck.SeverityFail:
			cliui.Fail(fmt.Sprintf("%s: %s", res.Name, res.Message))
		}
	}

	if report.Failed() {
		return fmt.Errorf("doctor found problems")
	}
	cliui.OK("doctor passed")
	return nil
}
