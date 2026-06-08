package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/config"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "config helpers (init, where, print)",
		// A bare `hop config` prints help (args empty → RunE shows usage). An
		// unknown subcommand (e.g. the deleted `config scan`) reaches this RunE
		// with args non-empty and surfaces an `unknown command` error rather than
		// silently printing help — cobra only routes a leftover positional to the
		// parent's RunE when the parent has no matching subcommand, so valid
		// subcommands (init/where/print and the hidden add/rm aliases) still
		// dispatch to their own RunE before this fires.
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		},
	}
	cmd.AddCommand(newConfigInitCmd(), newConfigWhereCmd(), newConfigAddCmd(), newConfigRmCmd(), newConfigPrintCmd())
	return cmd
}

func newConfigInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "bootstrap a starter hop.yaml at the resolved write target",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := config.ResolveWriteTarget()
			if err != nil {
				return err
			}
			if err := config.WriteStarter(target); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", target)
			fmt.Fprintln(cmd.ErrOrStderr(), "Edit the file to add your repos, or run `hop add -r <dir>` to populate from existing on-disk repos.")
			fmt.Fprintln(cmd.ErrOrStderr(), "Tip: to sync this config across machines, keep it in your dotfiles and symlink ~/.config/hop/hop.yaml to it.")
			return nil
		},
	}
}

func newConfigWhereCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "where",
		Short: "print the resolved hop.yaml path (regardless of file existence)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := config.ResolveWriteTarget()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), target)
			return nil
		},
	}
}

// newConfigPrintCmd returns the cobra factory for `hop config print`. The
// subcommand resolves the active hop.yaml via config.Resolve() — the same
// reader-contract resolver used by every other read path — then streams the
// file's bytes verbatim to stdout. Comments and formatting are preserved by
// construction; no parsing happens here.
func newConfigPrintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "print",
		Short: "print the resolved hop.yaml contents to stdout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Resolve()
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("hop config print: read %s: %w", path, err)
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
}
