package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Downloads information about your account and consumption to a local database",
	Run:   doSync,
}

var (
	tariff string
)

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().StringVar(&tariff, "tariff", "", "If set, specifies a tariff code to sync. Use the products command to list product codes.")
}

func doSync(command *cobra.Command, args []string) {
	ctx := context.Background()
	o, c := MustNewFromFlags(ctx)
	defer func() {
		if err := c(); err != nil {
			klog.Warningf("close: %v", err)
		}
	}()

	if tariff != "" {
		// TODO: fixme
		klog.Infof("Syncing tariff %s", tariff)
		if err := o.SyncTariff(ctx, tariff, fmt.Sprintf("E-1R-%s-J", tariff), time.Time{}, time.Now()); err != nil {
			klog.Exitf("SyncTariff(%s): %v", tariff, err)
		}
		return
	}

	if err := o.Sync(ctx); err != nil {
		klog.Exitf("Sync: %v", err)
	}
	_, _, err := o.Account(ctx)
	if err != nil {
		klog.Exitf("Account: %v", err)
	}
}
