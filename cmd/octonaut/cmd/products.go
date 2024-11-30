package cmd

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

// productsCmd represents the products command
var productsCmd = &cobra.Command{
	Use:   "products",
	Short: "List products and tariffs",
	Run:   doProducts,
}

func init() {
	rootCmd.AddCommand(productsCmd)
}

func doProducts(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	o, c := MustNewFromFlags(ctx)
	defer func() {
		if err := c(); err != nil {
			log.Warnf("close: %v", err)
		}
	}()

	ps, err := o.Products(ctx, nil)
	if err != nil {
		log.Fatalf("Products: %v", err)
	}
	for _, p := range ps.Results {
		log.Infof("%s:", p.Code)
		log.Infof("  %s", p.Description)
	}

}
