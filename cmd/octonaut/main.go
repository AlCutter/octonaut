package main

import (
	"context"
	"database/sql"
	"flag"
	"strings"
	"time"

	"github.com/AlCutter/octonaut/internal/octonaut"
	"github.com/AlCutter/octonaut/internal/octopus"
	_ "github.com/mattn/go-sqlite3"
	"k8s.io/klog/v2"
)

var (
	ep      = flag.String("endpoint", "https://api.octopus.energy/", "Base URL of the Octopus API.")
	account = flag.String("account", "", "Octopus Account e.g. A-123456.")
	key     = flag.String("key", "", "Octopus API key.")
	mpan    = flag.String("mpan", "", "Unique ID for an electricity meter.")
	dbPath  = flag.String("db", "./octonaut.sqlite3", "SQLite3 DB path and filename.")
)

func main() {
	flag.Parse()
	o := mustNewFromFlags()

	ctx := context.Background()

	a, err := o.Account(ctx)
	if err != nil {
		klog.Exitf("Account: %v", err)
	}
	klog.Infof("Account: %+v", a)

	var elecMeter *octopus.ElectricityMeterPoints
	var agreement *octopus.Agreement
	now := time.Now()

search:
	for _, p := range a.Properties {
		for _, e := range p.ElectricityMeterPoints {
			if len(e.ActiveMeters()) > 0 {
				agreement = e.ActiveAgreement(now)
				elecMeter = &e
				elecMeter.Meters = elecMeter.ActiveMeters()
				break search
			}
		}
	}
	if elecMeter == nil {
		klog.Exitf("Couldn't find electricity meterpoint with active meters")
	}

	klog.Infof("Agreement: %+v", agreement)

	products, err := o.Products(ctx, &agreement.ValidFrom)
	if err != nil {
		klog.Exitf("Products: %v", err)
	}
	tariffProduct := products.FindByTariff(agreement.TariffCode)
	if tariffProduct == nil {
		klog.Exitf("Couldn't find product for code %q", agreement.TariffCode)
	}
	klog.Infof("Tariff Product: %+v", tariffProduct)
	/*
		cons, err := c.Consumption(ctx, elecMeter.MPAN, elecMeter.Meters[0].SerialNumber, now.Add(-30*24*time.Hour), now)
		if err != nil {
			klog.Exitf("Consumption: %v", err)
		}
		klog.Infof("Cons: %+v", cons)
	*/

	_, err = o.TariffRates(ctx, tariffProduct.Code, "electricity", agreement.TariffCode, "standard-unit-rates", now.Add(-30*24*time.Hour), now)
	if err != nil {
		klog.Exitf("Tariff: %v", err)
	}
	//	klog.Infof("Tariff: %+v", tariff)
}

func mustNewFromFlags() *octonaut.Octonaut {
	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		klog.Exitf("Failed to open DB (%q): %v", *dbPath, err)
	}
	defer db.Close()

	u := *ep
	if !strings.HasSuffix(u, "/") {
		u += "/"
	}

	r := octonaut.New(*account, *key, u, db)

	return r
}
