// Package cli is claude-switch's command line: it parses arguments, wires the
// adapters for this OS into the app service and renders its results.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/PuntoyComaTech/claude-switch/internal/app"
)

// Execute runs the command line and returns the process exit code.
func Execute(ctx context.Context, version string) int {
	root := newRoot(version)
	if err := root.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "claude-switch: %v\n", err)
		return 1
	}
	return 0
}

func newRoot(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "claude-switch",
		Short: "Switch Claude Code between Claude subscription accounts",
		Long: "claude-switch saves several Claude Pro/Max/Team logins and swaps the one Claude Code uses,\n" +
			"automatically when the active account hits its 5-hour or weekly usage limit.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// svc builds the service lazily so --help never touches the system.
	build := func(out io.Writer) (*app.Service, error) {
		e, err := loadEnv()
		if err != nil {
			return nil, err
		}
		return newService(e, version, out)
	}
	svc := func() (*app.Service, error) { return build(os.Stdout) }
	// The status line's stdout is what Claude Code displays, so switch
	// messages must not reach it.
	quiet := func() (*app.Service, error) { return build(io.Discard) }
	root.AddCommand(
		addCmd(svc), useCmd(svc), nextCmd(svc), loginCmd(svc), listCmd(svc),
		currentCmd(svc), thresholdCmd(svc), renameCmd(svc), removeCmd(svc),
		reportCmd(quiet), statuslineCmd(quiet), setupCmd(),
	)
	return root
}

type svcFn func() (*app.Service, error)

// with adapts a use case to cobra's RunE.
func with(svc svcFn, fn func(context.Context, *app.Service, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		s, err := svc()
		if err != nil {
			return err
		}
		return fn(cmd.Context(), s, args)
	}
}

func addCmd(svc svcFn) *cobra.Command {
	var login bool
	c := &cobra.Command{
		Use:   "add <name>",
		Short: "Save the account you are logged in with (or --login to add another one)",
		Args:  cobra.ExactArgs(1),
		RunE: with(svc, func(ctx context.Context, s *app.Service, a []string) error {
			if login {
				return s.AddWithLogin(ctx, a[0])
			}
			return s.Add(ctx, a[0])
		}),
	}
	c.Flags().BoolVar(&login, "login", false, "log in to another account in the browser without touching running sessions")
	return c
}

func useCmd(svc svcFn) *cobra.Command {
	return &cobra.Command{
		Use: "use <name>", Short: "Switch to a saved account (logs in again if its token expired)",
		Args: cobra.ExactArgs(1),
		RunE: with(svc, func(ctx context.Context, s *app.Service, a []string) error { return s.Use(ctx, a[0], false) }),
	}
}

func nextCmd(svc svcFn) *cobra.Command {
	return &cobra.Command{
		Use: "next", Short: "Switch to the next account that is not at its limit", Args: cobra.NoArgs,
		RunE: with(svc, func(ctx context.Context, s *app.Service, _ []string) error { return s.Next(ctx, false) }),
	}
}

func loginCmd(svc svcFn) *cobra.Command {
	return &cobra.Command{
		Use: "login <name>", Short: "Log in again to a saved account", Args: cobra.ExactArgs(1),
		RunE: with(svc, func(ctx context.Context, s *app.Service, a []string) error { return s.Login(ctx, a[0]) }),
	}
}

func listCmd(svc svcFn) *cobra.Command {
	return &cobra.Command{
		Use: "list", Aliases: []string{"ls", "status"}, Short: "Show accounts and their usage", Args: cobra.NoArgs,
		RunE: with(svc, func(ctx context.Context, s *app.Service, _ []string) error {
			v, err := s.List(ctx)
			if err != nil {
				return err
			}
			renderList(os.Stdout, v, time.Now(), useColor(os.Stdout))
			return nil
		}),
	}
}

func currentCmd(svc svcFn) *cobra.Command {
	return &cobra.Command{
		Use: "current", Short: "Print the active account name", Args: cobra.NoArgs,
		RunE: with(svc, func(_ context.Context, s *app.Service, _ []string) error { return s.Current() }),
	}
}

func thresholdCmd(svc svcFn) *cobra.Command {
	return &cobra.Command{
		Use: "threshold [percent]", Short: "Show or set the usage % that triggers a switch (default 95)",
		Args: cobra.MaximumNArgs(1),
		RunE: with(svc, func(ctx context.Context, s *app.Service, a []string) error {
			v := ""
			if len(a) == 1 {
				v = a[0]
			}
			return s.Threshold(ctx, v)
		}),
	}
}

func renameCmd(svc svcFn) *cobra.Command {
	return &cobra.Command{
		Use: "rename <old> <new>", Aliases: []string{"mv"}, Short: "Rename a saved account", Args: cobra.ExactArgs(2),
		RunE: with(svc, func(ctx context.Context, s *app.Service, a []string) error { return s.Rename(ctx, a[0], a[1]) }),
	}
}

func removeCmd(svc svcFn) *cobra.Command {
	return &cobra.Command{
		Use: "remove <name>", Aliases: []string{"rm"}, Short: "Delete a saved account", Args: cobra.ExactArgs(1),
		RunE: with(svc, func(ctx context.Context, s *app.Service, a []string) error { return s.Remove(ctx, a[0]) }),
	}
}

func reportCmd(svc svcFn) *cobra.Command {
	return &cobra.Command{
		Use:   "report",
		Short: "Read Claude Code status line JSON on stdin; switch when over the limit",
		Args:  cobra.NoArgs,
		RunE: with(svc, func(ctx context.Context, s *app.Service, _ []string) error {
			var in statusInput
			if err := json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&in); err != nil {
				return fmt.Errorf("read status line JSON: %w", err)
			}
			return s.Report(ctx, in.StatusLine)
		}),
	}
}

// statuslineCmd is a ready-made status line: it reports usage and prints
// "account · model · 5h · 7d". It never fails, so the status line always renders.
func statuslineCmd(svc svcFn) *cobra.Command {
	return &cobra.Command{
		Use:   "statusline",
		Short: "Built-in Claude Code status line (reports usage and shows the account)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var in statusInput
			_ = json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&in)
			account := ""
			if s, err := svc(); err == nil {
				_ = s.Report(cmd.Context(), in.StatusLine)
				account = currentName(s)
			}
			_, _ = fmt.Fprint(os.Stdout, statusText(account, in))
			return nil
		},
	}
}

func setupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Hook claude-switch into the Claude Code status line",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			e, err := loadEnv()
			if err != nil {
				return err
			}
			return setupStatusLine(os.Stdout, e.settingsPath())
		},
	}
}

func currentName(s *app.Service) string {
	name, err := s.ActiveName()
	if err != nil {
		return ""
	}
	return name
}

func useColor(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
