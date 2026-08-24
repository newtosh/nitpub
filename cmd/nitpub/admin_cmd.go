package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/newtosh/nitpub/internal/auth"
	"github.com/newtosh/nitpub/internal/config"
	"github.com/newtosh/nitpub/internal/store"
)

type adminEnv struct {
	svc     *auth.Service
	cfg     config.Config
	cleanup func()
}

func newAdminCmd() *cobra.Command {
	var offline bool

	admin := &cobra.Command{
		Use:   "admin",
		Short: "Manage the admin account",
		Long: `Manage the single admin account (password, 2FA, backup codes).

Admin commands open the bbolt database directly. While the nitpub service is
running it holds an exclusive lock, so commands fail fast unless you pass --offline
(which stops the service, runs the command, then starts it again).`,
	}

	admin.PersistentFlags().BoolVar(&offline, "offline", false,
		"stop nitpub.service before running, then start it again (requires root)")

	admin.AddCommand(
		newAdminInitCmd(&offline),
		newAdminStatusCmd(&offline),
		newAdminPasswordCmd(&offline),
		newAdminTOTPCmd(&offline),
		newAdminWebAuthnCmd(&offline),
		newAdminBackupCodesCmd(&offline),
		newAdminReset2FACmd(&offline),
	)

	return admin
}

func openAdminEnv(offline bool) (*adminEnv, error) {
	env := &adminEnv{cleanup: func() {}}
	if offline {
		stopped, err := stopNitpubService()
		if err != nil {
			return nil, err
		}
		if stopped {
			env.cleanup = func() {
				_ = startNitpubService()
			}
		}
	}

	cfg, err := config.Load()
	if err != nil {
		env.cleanup()
		return nil, err
	}
	if err := config.EnsureDataDirWritable(cfg); err != nil {
		env.cleanup()
		return nil, err
	}

	timeout := 3 * time.Second
	if offline {
		timeout = 0
	}
	st, err := store.OpenWithTimeout(cfg.DataDir, timeout)
	if err != nil {
		env.cleanup()
		if errors.Is(err, store.ErrDatabaseLocked) {
			svcName := nitpubServiceName()
			return nil, fmt.Errorf("%w\n\nhint: nitpub admin … --offline   (as root, stops the service)\n      systemctl stop %s && sudo -u nitpub nitpub admin …", err, svcName)
		}
		return nil, err
	}

	svc, err := auth.NewService(st, cfg.Domain, "nitpub")
	if err != nil {
		_ = st.Close()
		env.cleanup()
		return nil, err
	}
	svc.SetRPOrigin(cfg.BaseURL)

	prev := env.cleanup
	env.cleanup = func() {
		_ = st.Close()
		prev()
	}
	env.svc = svc
	env.cfg = cfg
	return env, nil
}

func withAdminEnv(offline *bool, fn func(*adminEnv) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		env, err := openAdminEnv(*offline)
		if err != nil {
			return err
		}
		defer env.cleanup()
		return fn(env)
	}
}

func newAdminInitCmd(offline *bool) *cobra.Command {
	var username, password string
	var passwordStdin, force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create the admin account",
		Example: `  nitpub admin init --username you@example.com --offline
  nitpub admin init --username you --password-stdin --offline <<< 'secret'
  nitpub admin init --username you@example.com --force --offline   # replace an existing admin`,
		RunE: withAdminEnv(offline, func(env *adminEnv) error {
			pw, err := passwordFromFlags(password, passwordStdin)
			if err != nil {
				return err
			}
			if err := env.svc.InitAdmin(username, pw, force); err != nil {
				return err
			}
			fmt.Printf("Admin user %q created.\n", username)
			return nil
		}),
	}

	cmd.Flags().StringVar(&username, "username", "admin", "admin username")
	cmd.Flags().StringVar(&password, "password", "", "password (non-interactive; avoid on shared hosts)")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read password from stdin (no confirmation)")
	cmd.Flags().BoolVar(&force, "force", false, "replace an existing admin account (drops its TOTP/WebAuthn/backup codes)")
	return cmd
}

func newAdminStatusCmd(offline *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether an admin account is configured",
		RunE: withAdminEnv(offline, func(env *adminEnv) error {
			exists, err := env.svc.Store().AdminExists()
			if err != nil {
				return err
			}
			if !exists {
				fmt.Println("Admin: not configured")
				fmt.Println("Run: nitpub admin init --username you --offline")
				return nil
			}
			rec, err := env.svc.Store().GetAdmin()
			if err != nil {
				return err
			}
			totp := "off"
			if rec.Settings.TOTPEnabled {
				totp = "on"
			}
			webauthn := "off"
			if rec.Settings.WebAuthnEnabled {
				webauthn = "on"
			}
			backups := len(rec.BackupCodeHashes)
			fmt.Printf("Admin: %s\n", rec.Username)
			fmt.Printf("TOTP: %s  WebAuthn: %s  Backup codes: %d\n", totp, webauthn, backups)
			if env.cfg.ConfigPath != "" {
				fmt.Printf("Config: %s\n", env.cfg.ConfigPath)
			}
			fmt.Printf("Data: %s\n", env.cfg.DataDir)
			return nil
		}),
	}
}

func newAdminPasswordCmd(offline *bool) *cobra.Command {
	var force bool
	var newPassword string
	var newPasswordStdin bool

	cmd := &cobra.Command{
		Use:   "password",
		Short: "Change the admin password",
		RunE: withAdminEnv(offline, func(env *adminEnv) error {
			var current string
			var err error
			if !force {
				current, err = readPassword("Current password: ")
				if err != nil {
					return err
				}
			}
			var newPw string
			switch {
			case newPassword != "":
				newPw = newPassword
			case newPasswordStdin:
				newPw, err = passwordFromFlags("", true)
			default:
				newPw, err = readPasswordTwice("New password: ", "Confirm password: ")
			}
			if err != nil {
				return err
			}
			if err := env.svc.ChangePassword(current, newPw, force); err != nil {
				return err
			}
			fmt.Println("Password updated.")
			return nil
		}),
	}

	cmd.Flags().BoolVar(&force, "force", false, "skip current password check")
	cmd.Flags().StringVar(&newPassword, "password", "", "new password")
	cmd.Flags().BoolVar(&newPasswordStdin, "password-stdin", false, "read new password from stdin")
	return cmd
}

func newAdminTOTPCmd(offline *bool) *cobra.Command {
	totp := &cobra.Command{
		Use:   "totp",
		Short: "Enable or disable TOTP",
	}
	totp.AddCommand(
		&cobra.Command{
			Use:   "enable",
			Short: "Enable TOTP for the admin account",
			RunE: withAdminEnv(offline, func(env *adminEnv) error {
				rec, err := env.svc.Store().GetAdmin()
				if err != nil {
					return err
				}
				secret, url, err := env.svc.Store().EnableTOTP("nitpub", rec.Username)
				if err != nil {
					return err
				}
				fmt.Println("TOTP enabled. Scan this URL in your authenticator app:")
				fmt.Println(url)
				fmt.Println("Secret:", auth.FormatTOTPSecretForDisplay(secret))
				fmt.Println("Issuer:", env.cfg.Domain)
				return nil
			}),
		},
		&cobra.Command{
			Use:   "disable",
			Short: "Disable TOTP",
			RunE: withAdminEnv(offline, func(env *adminEnv) error {
				if err := env.svc.Store().DisableTOTP(); err != nil {
					return err
				}
				fmt.Println("TOTP disabled.")
				return nil
			}),
		},
	)
	return totp
}

func newAdminWebAuthnCmd(offline *bool) *cobra.Command {
	webauthn := &cobra.Command{
		Use:   "webauthn",
		Short: "Register or disable WebAuthn passkeys",
	}
	webauthn.AddCommand(
		&cobra.Command{
			Use:   "register",
			Short: "Print a browser enrollment URL",
			RunE: withAdminEnv(offline, func(env *adminEnv) error {
				et, err := auth.NewEnrollToken(auth.NowUTC())
				if err != nil {
					return err
				}
				if err := env.svc.Store().PutEnrollToken(et); err != nil {
					return err
				}
				fmt.Printf("Open this URL in your browser within 10 minutes:\n%s/author/enroll?token=%s\n", env.cfg.BaseURL, et.Token)
				return nil
			}),
		},
		&cobra.Command{
			Use:   "disable",
			Short: "Disable WebAuthn",
			RunE: withAdminEnv(offline, func(env *adminEnv) error {
				if err := env.svc.DisableWebAuthn(); err != nil {
					return err
				}
				fmt.Println("WebAuthn disabled.")
				return nil
			}),
		},
	)
	return webauthn
}

func newAdminBackupCodesCmd(offline *bool) *cobra.Command {
	backup := &cobra.Command{
		Use:   "backup-codes",
		Short: "Manage backup codes",
	}
	backup.AddCommand(&cobra.Command{
		Use:   "regenerate",
		Short: "Generate new backup codes",
		RunE: withAdminEnv(offline, func(env *adminEnv) error {
			codes, err := env.svc.Store().RegenerateBackupCodes()
			if err != nil {
				return err
			}
			fmt.Println(auth.BackupCodesHelp())
			fmt.Println(auth.FormatBackupCodes(codes))
			return nil
		}),
	})
	return backup
}

func newAdminReset2FACmd(offline *bool) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "reset-2fa",
		Short: "Disable TOTP, WebAuthn, and backup codes",
		RunE: withAdminEnv(offline, func(env *adminEnv) error {
			if !force {
				current, err := readPassword("Current password: ")
				if err != nil {
					return err
				}
				rec, err := env.svc.Store().GetAdmin()
				if err != nil {
					return err
				}
				if !auth.VerifyPassword(current, rec.PasswordHash) {
					return fmt.Errorf("current password incorrect")
				}
			}
			if err := env.svc.Reset2FA(); err != nil {
				return err
			}
			fmt.Println("2FA and backup codes reset.")
			return nil
		}),
	}
	cmd.Flags().BoolVar(&force, "force", false, "skip current password check")
	return cmd
}
