package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/newtosh/nitpub/internal/api"
	"github.com/newtosh/nitpub/internal/config"
	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/store"
)

func newImportCmd() *cobra.Command {
	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import content into nitpub",
	}
	importCmd.AddCommand(newImportPostsCmd())
	return importCmd
}

func newImportPostsCmd() *cobra.Command {
	var kind string
	var offline bool

	cmd := &cobra.Command{
		Use:   "posts <dir>",
		Short: "Import markdown files from a directory as posts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImportPosts(args[0], kind, offline)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "article", "default post kind (note or article)")
	cmd.Flags().BoolVar(&offline, "offline", false,
		"stop nitpub.service before running, then start it again (requires root)")
	return cmd
}

func runImportPosts(dir, kind string, offline bool) error {
	if offline {
		stopped, err := stopNitpubService()
		if err != nil {
			return err
		}
		if stopped {
			defer func() { _ = startNitpubService() }()
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		return err
	}
	defer st.Close()

	defaultKind := outbox.Kind(strings.ToLower(kind))
	if defaultKind != outbox.KindNote && defaultKind != outbox.KindArticle {
		return fmt.Errorf("invalid kind %q", kind)
	}

	ob := outbox.New(st, cfg.BaseURL, cfg.BaseURL+"/actor")
	resp, err := api.ImportPostsFromDir(ob, nil, dir, defaultKind)
	if err != nil {
		return err
	}
	fmt.Printf("imported %d post(s)\n", resp.Imported)
	for _, e := range resp.Errors {
		fmt.Printf("error: %s\n", e)
	}
	if resp.Imported == 0 && len(resp.Errors) > 0 {
		return fmt.Errorf("import failed")
	}
	return nil
}
