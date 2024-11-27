package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
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
			klog.Warningf("close: %v", err)
		}
	}()

	ps, err := o.Products(ctx, nil)
	if err != nil {
		klog.Exitf("Products: %v", err)
	}
	for _, p := range ps.Results {
		klog.Infof("%s:", p.Code)
		klog.Infof("  %s:", p.Description)
	}

}
