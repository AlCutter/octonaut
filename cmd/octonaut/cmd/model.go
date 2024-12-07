package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AlCutter/octonaut/internal/octonaut"
	"github.com/AlCutter/octonaut/internal/octopus"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	_ "github.com/mattn/go-sqlite3"
)

// modelCmd represents the sync command
var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Allows you to model various scenarios and calculate potential differences in costs",
	Run:   doModel,
}

var (
	batteryCap    float64
	batteryRate   float64
	batteryCharge string

	fromStr string
	toStr   string
)

func init() {
	rootCmd.AddCommand(modelCmd)

	modelCmd.Flags().StringVar(&tariff, "tariff", "", "Tariff code to use for modelling.")
	modelCmd.Flags().Float64Var(&batteryCap, "battery_capacity", 0, "Battery capacity in kWh for modelling load shifting.")
	modelCmd.Flags().Float64Var(&batteryRate, "battery_rate", 0, "Battery max charge/discharge rate in kWh for modelling load shifting.")
	modelCmd.Flags().StringVar(&batteryCharge, "battery_charge", "", "Battery charge stratech for load shifting. Valid options: <hour>-<hour> (e.g. '0-5' to charge between midnight and 5am).")

	modelCmd.Flags().StringVar(&fromStr, "from", "", "Date from which to start modelling (YYYY-MM-DD).")
	modelCmd.Flags().StringVar(&toStr, "to", "", "Date to model to, or leave until to model until today (YYYY-MM-DD).")

	modelCmd.MarkFlagsRequiredTogether("battery_capacity", "battery_rate", "battery_charge")
	modelCmd.MarkFlagRequired("from")
	modelCmd.MarkFlagRequired("tariff")
}

func doModel(command *cobra.Command, args []string) {
	ctx := context.Background()
	o, c := MustNewFromFlags(ctx)
	defer func() {
		if err := c(); err != nil {
			log.Warnf("close: %v", err)
		}
	}()

	a, _, err := o.Account(ctx)
	if err != nil {
		log.Fatalf("Account: %v", err)
	}

	ps := a.Properties[0]
	em := ps.ElectricityMeterPoints[0]

	from, err := time.Parse(time.DateOnly, fromStr)
	if err != nil {
		log.Fatalf("Invalid from date: %v", err)
	}
	to := time.Now().Truncate(24 * time.Hour)
	if toStr != "" {
		to, err = time.Parse(time.DateOnly, toStr)
		if err != nil {
			log.Fatalf("Invalid to date: %v", err)
		}
	}
	log.Infof("From: %v", from)
	log.Infof("To: %v", to)

	// TODO
	cons, err := o.Consumption(ctx, em.MPAN, em.Meters[0].SerialNumber, from, to)
	if err != nil {
		log.Fatalf("Consumption: %v", err)
	}

	const (
		agile24       = "AGILE-24-04-03"
		agile23       = "AGILE-23-12-06"
		intelligentGo = "INTELLI-VAR-22-10-14"
	)

	agreement := em.ActiveAgreement(time.Now())
	f, r, _, pc, err := octopus.ParseTariffCode(agreement.TariffCode)
	if err != nil {
		log.Fatalf("Failed to parse existing tariff code: %v", err)
	}

	tariffCode := octopus.BuildTariffCode(f, r, tariff, pc)
	log.Infof("Using TariffCode %q", tariffCode)
	if err := o.SyncTariff(ctx, tariff, tariffCode, from, to); err != nil {
		log.Fatalf("SyncTariff (%s): %v", tariff, err)
	}
	rates, err := o.TariffRates(ctx, tariffCode, from, to)
	if err != nil {
		log.Fatalf("TariffRates: %v", err)
	}

	if batteryCharge != "" {
		cs, err := chargeStrategy(batteryCharge)
		if err != nil {
			log.Fatalf("Invalid battery charge strategy: %v", err)
		}
		loadShift, _ := octonaut.LoadShift(batteryCap, batteryRate, 0, cs)
		cons = octonaut.Apply(loadShift, cons)
	}

	_ = runModel(ctx, cons, rates)
	/*
		if err := writeCSV("orig.csv", origCost); err != nil {
			log.Fatalf("writeCSV: %v", err)
		}
	*/

}

func runModel(ctx context.Context, cons octonaut.Consumption, rates *octopus.TariffRate) *octonaut.Cost {
	start := cons.Intervals[0].Start
	end := cons.Intervals[len(cons.Intervals)-1].End
	cost, err := octonaut.TotalCost(ctx, cons, octonaut.Tariff(*rates))
	if err != nil {
		log.Fatalf("TotalCost: %v", err)
	}
	log.Infof("Energy    : £%.2f (inc. VAT) (%.2f kWh)", cost.TotalCost/100.0, cost.TotalConsumption)
	days := float64((end.Sub(start)) / (24 * time.Hour))
	standing := 54.83 * days
	log.Infof("Standing  : £%.2f (inc. VAT) (%.1f days)", standing/100.0, days)
	totalCost := (cost.TotalCost + standing)
	log.Infof("Total Cost: £%.2f (£%.2f/day, effective £%.2f/kWh)", totalCost/100.0, (totalCost/100.0)/days, (totalCost/100.0)/cost.TotalConsumption)

	return cost
}

func writeCSV(name string, c *octonaut.Cost, s ...octonaut.IntervalStat) error {
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("Open(%q): %v", name, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Fatalf("Close(%q): %v", name, err)
		}
	}()
	if err := c.ToCSV(f, s...); err != nil {
		return fmt.Errorf("ToCSV: %v", err)
	}
	return nil
}

func chargeStrategy(s string) (func(t time.Time) bool, error) {
	bits := strings.Split(s, "-")
	if len(bits) != 2 {
		return nil, fmt.Errorf("invalid strategy format, must be <N>-<M>")
	}
	n, err := parseHour(bits[0])
	if err != nil {
		return nil, fmt.Errorf("interval start: %v", err)
	}
	m, err := parseHour(bits[1])
	if err != nil {
		return nil, fmt.Errorf("interval end: %v", err)
	}
	return func(t time.Time) bool {
		h := float64(t.Hour()) + float64(t.Minute())/60.0
		if n <= m {
			// range is within a single day, e.g. 5-10
			return h >= n && h < m
		} else {
			// range crosses midnight boundary, e.g. 23-4
			return h >= n || h < m
		}
	}, nil
}

func parseHour(s string) (float64, error) {
	i, err := strconv.ParseFloat(s, 5)
	if err != nil {
		return 0, err
	}
	if i < 0 || i >= 24 {
		return 0, fmt.Errorf("%f should be 0 <= N < 24", i)
	}
	return i, nil
}
