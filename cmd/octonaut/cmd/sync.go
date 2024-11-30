package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
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
			log.Warnf("close: %v", err)
		}
	}()

	if tariff != "" {
		// TODO: fixme
		log.Infof("Syncing tariff %s", tariff)
		if err := o.SyncTariff(ctx, tariff, fmt.Sprintf("E-1R-%s-J", tariff), time.Time{}, time.Now()); err != nil {
			log.Fatalf("SyncTariff(%s): %v", tariff, err)
		}
		return
	}

	if err := o.Sync(ctx); err != nil {
		log.Fatalf("Sync: %v", err)
	}
	_, _, err := o.Account(ctx)
	if err != nil {
		log.Fatalf("Account: %v", err)
	}
}
