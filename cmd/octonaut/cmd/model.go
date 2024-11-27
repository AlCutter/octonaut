package cmd

import (
	"context"
	"fmt"
	"os"
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
	o, c := MustNewFromFlags(ctx)
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
	//start := time.Date(now.Year(), time.March, 19, 0, 0, 0, 0, time.Local)
	start := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.Local)
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

	const (
		agile24       = "AGILE-24-04-03"
		agile23       = "AGILE-23-12-06"
		intelligentGo = "INTELLI-VAR-22-10-14"
	)

	if err := o.SyncTariff(ctx, agile24, fmt.Sprintf("E-1R-%s-J", agile24), start, end); err != nil {
		klog.Exitf("SyncTariff (%s): %v", agile24, err)
	}
	if err := o.SyncTariff(ctx, agile23, fmt.Sprintf("E-1R-%s-J", agile23), start, end); err != nil {
		klog.Exitf("SyncTariff (%s): %v", agile23, err)
	}
	if err := o.SyncTariff(ctx, "INTELLI-VAR-22-10-14", "E-1R-INTELLI-VAR-22-10-14-J", start, end); err != nil {
		klog.Exitf("SyncTariff (intelli): %v", err)
	}

	agileRates, err := o.TariffRates(ctx, fmt.Sprintf("E-1R-%s-J", agile23), start, end)
	if err != nil {
		klog.Exitf("Tariff (%s): %v", agile23, err)
	}

	intelliRates, err := o.TariffRates(ctx, "E-1R-INTELLI-VAR-22-10-14-J", start, end)
	if err != nil {
		klog.Exitf("Tariff (intelli): %v", err)
	}

	//cost, err := octonaut.TotalCost(ctx, cons, octonaut.FlatRate(0.25))
	loadShift, shiftStats := octonaut.LoadShift(60, 10, 60, 0, 5)
	shiftedCons := octonaut.Apply(loadShift, *cons)

	klog.Infof("Unmodified=============")
	origCost := runModel(ctx, cons, agileRates)
	if err := writeCSV("orig.csv", origCost); err != nil {
		klog.Exitf("writeCSV: %v", err)
	}

	klog.Infof("Load shifted (%s)===========", agile23)
	shiftedAgileCost := runModel(ctx, &shiftedCons, agileRates)
	klog.Infof("Saving shifted Agile: £%.2f", (origCost.TotalCost-shiftedAgileCost.TotalCost)/100.0)
	if err := writeCSV("agile_shifted.csv", shiftedAgileCost, shiftStats); err != nil {
		klog.Exitf("writeCSV: %v", err)
	}

	klog.Infof("Load shifted (intelli)===========")
	shiftedIntelliCost := runModel(ctx, &shiftedCons, intelliRates)
	klog.Infof("Saving shifted Intelligent Go: £%.2f", (origCost.TotalCost-shiftedIntelliCost.TotalCost)/100.0)
	if err := writeCSV("intelligentgo_shifted.csv", shiftedIntelliCost, shiftStats); err != nil {
		klog.Exitf("writeCSV: %v", err)
	}

}

func runModel(ctx context.Context, cons *octonaut.Consumption, rates *octopus.TariffRate) *octonaut.Cost {
	start := cons.Intervals[0].Start
	end := cons.Intervals[len(cons.Intervals)-1].End
	cost, err := octonaut.TotalCost(ctx, cons, octonaut.Tariff(*rates))
	if err != nil {
		klog.Exitf("TotalCost: %v", err)
	}
	klog.Infof("Energy    : £%.2f (inc. VAT) (%.2f kWh)", cost.TotalCost/100.0, cost.TotalConsumption)
	days := float64((end.Sub(start)) / (24 * time.Hour))
	standing := 54.83 * days
	klog.Infof("Standing  : £%.2f (inc. VAT) (%.1f days)", standing/100.0, days)
	totalCost := (cost.TotalCost + standing)
	klog.Infof("Total Cost: £%.2f (£%.2f/day, effective £%.2f/kWh)", totalCost/100.0, (totalCost/100.0)/days, (totalCost / 100.0))

	return cost
}

func writeCSV(name string, c *octonaut.Cost, s ...octonaut.IntervalStat) error {
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("Open(%q): %v", name, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			klog.Exitf("Close(%q): %v", name, err)
		}
	}()
	if err := c.ToCSV(f, s...); err != nil {
		return fmt.Errorf("ToCSV: %v", err)
	}
	return nil

}
