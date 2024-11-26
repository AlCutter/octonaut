package cmd

import (
	"context"
	"time"

	"github.com/AlCutter/octonaut/internal/octonaut"
	"github.com/AlCutter/octonaut/internal/octopus"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	_ "github.com/mattn/go-sqlite3"
)

// modelCmd represents the sync command
var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Allows you to model various scenarios and calculate potential differences in costs",
	Run:   doModel,
}

func init() {
	rootCmd.AddCommand(modelCmd)
}

func doModel(command *cobra.Command, args []string) {
	ctx := context.Background()
	c, o := MustNewFromFlags(ctx)
	defer func() {
		if err := c(); err != nil {
			klog.Warningf("close: %v", err)
		}
	}()

	a, _, err := o.Account(ctx)
	if err != nil {
		klog.Exitf("Account: %v", err)
	}

	ps := a.Properties[0]
	em := ps.ElectricityMeterPoints[0]

	//	now := time.Now().Add(-24 * time.Hour * 3).Truncate(30 * time.Minute)
	//	end := now.Add(8 * time.Hour)
	now := time.Now().Truncate(24 * time.Hour).UTC()
	start := time.Date(now.Year(), time.March, 19, 0, 0, 0, 0, time.Local)
	//start := time.Date(now.Year(), time.October, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(now.Year(), time.October, 31, 23, 59, 59, 0, time.Local)
	//end := now.Add(-24 * time.Hour * 3)
	klog.Infof("Start: %v", start)
	klog.Infof("End: %v", end)
	//end := now.Add(15 * time.Minute)
	cons, err := o.Consumption(ctx, em.MPAN, em.Meters[0].SerialNumber, start, end)
	if err != nil {
		klog.Exitf("Consumption: %v", err)
	}

	if err := o.SyncTariff(ctx, "AGILE-24-04-03", "E-1R-AGILE-24-04-03-J", start, end); err != nil {
		klog.Exitf("SyncTariff (agile): %v", err)
	}
	if err := o.SyncTariff(ctx, "INTELLI-VAR-22-10-14", "E-1R-INTELLI-VAR-22-10-14-J", start, end); err != nil {
		klog.Exitf("SyncTariff (intelli): %v", err)
	}

	agileRates, err := o.TariffRates(ctx, "E-1R-AGILE-24-04-03-J", start, end)
	if err != nil {
		klog.Exitf("Tariff (agile): %v", err)
	}

	intelliRates, err := o.TariffRates(ctx, "E-1R-INTELLI-VAR-22-10-14-J", start, end)
	if err != nil {
		klog.Exitf("Tariff (intelli): %v", err)
	}

	//cost, err := octonaut.TotalCost(ctx, cons, octonaut.FlatRate(0.25))
	shiftedCons := octonaut.Apply(octonaut.LoadShift(40, 10, 60, 0, 5), *cons)

	klog.Infof("Unmodified=============")
	totalOrig := runModel(ctx, cons, agileRates)

	klog.Infof("Load shifted (agile)===========")
	totalShiftedAgile := runModel(ctx, &shiftedCons, agileRates)
	klog.Infof("Saving shifted Agile: £%.2f", (totalOrig-totalShiftedAgile)/100.0)

	klog.Infof("Load shifted (intelli)===========")
	totalShiftedIntelli := runModel(ctx, &shiftedCons, intelliRates)
	klog.Infof("Saving shifted Intelligent Go: £%.2f", (totalOrig-totalShiftedIntelli)/100.0)

}

func runModel(ctx context.Context, cons *octonaut.Consumption, rates *octopus.TariffRate) float64 {
	start := cons.Intervals[0].Start
	end := cons.Intervals[len(cons.Intervals)-1].End
	energyCost, energyCons, err := octonaut.TotalCost(ctx, cons, octonaut.Tariff(*rates))
	if err != nil {
		klog.Exitf("TotalCost: %v", err)
	}
	klog.Infof("Energy    : £%.2f (inc. VAT) (%.2f kWh)", energyCost/100.0, energyCons)
	days := float64((end.Sub(start)) / (24 * time.Hour))
	standing := 54.83 * days
	klog.Infof("Standing  : £%.2f (inc. VAT) (%.1f days)", standing/100.0, days)
	totalCost := (energyCost + standing)
	klog.Infof("Total Cost: £%.2f", totalCost/100.0)

	return totalCost

}
