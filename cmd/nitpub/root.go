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
			return run()
		},
	}
	root.AddCommand(newAdminCmd())
	root.AddCommand(newFederationCmd())
	root.AddCommand(newImportCmd())
	root.AddCommand(newUpdateCmd())
	root.AddCommand(newInstallCmd())
	root.AddCommand(newDoctorCmd())
	return root
}
