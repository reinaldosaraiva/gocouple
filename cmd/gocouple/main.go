package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "gocouple",
		Short:         "Coupling metrics and architectural health over time for Go",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCmd(), newAnalyzeCmd(), newHistoryCmd(), newReportCmd(), newCheckCmd())
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit and build date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "gocouple %s (commit %s, built %s)\n", version, commit, date)
			return err
		},
	}
}

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		stop()
	}()
	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		var ec *exitCodeError
		if errors.As(err, &ec) {
			if ec.err != nil {
				fmt.Fprintln(os.Stderr, "gocouple:", ec.err)
			}
			return ec.code
		}
		fmt.Fprintln(os.Stderr, "gocouple:", err)
		return 1
	}
	return 0
}
