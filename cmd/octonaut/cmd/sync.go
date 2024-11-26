package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Downloads information about your account and consumption to a local database",
	Run:   doSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func doSync(command *cobra.Command, args []string) {
	ctx := context.Background()
	c, o := MustNewFromFlags(ctx)
	defer func() {
		if err := c(); err != nil {
			klog.Warningf("close: %v", err)
		}
	}()

	if err := o.Sync(ctx); err != nil {
		klog.Exitf("Sync: %v", err)
	}
	_, _, err := o.Account(ctx)
	if err != nil {
		klog.Exitf("Account: %v", err)
	}
}
