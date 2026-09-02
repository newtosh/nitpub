package main

import (
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "nitpub",
		Short: "nitpub ActivityPub blog server",
		Long:  "Single-binary ActivityPub blog and admin CLI.",
		RunE: func(cmd *cobra.Command, args []string) error {
			enableQuotePosts, err := cmd.Flags().GetBool("enable-quote-posts")
			if err != nil {
				return err
			}
			return run(enableQuotePosts)
		},
	}
	root.PersistentFlags().Bool("enable-quote-posts", false, "enable the quote-post feature (CLI-only; cannot be set via config.toml)")
	root.AddCommand(newAdminCmd())
	root.AddCommand(newFederationCmd())
	root.AddCommand(newImportCmd())
	root.AddCommand(newUpdateCmd())
	root.AddCommand(newInstallCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newTelemetryCmd())
	return root
}
