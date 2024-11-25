package main

import (
	"context"
	"database/sql"
	"flag"
	"strings"
	"time"

	"github.com/AlCutter/octonaut/internal/octonaut"
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
	klog.InitFlags(nil)
	flag.Parse()

	ctx := context.Background()
	c, o := mustNewFromFlags(ctx)
	defer func() {
		if err := c(); err != nil {
			klog.Warningf("close: %v", err)
		}
	}()

	a, needSync, err := o.Account(ctx)
	if err != nil {
		klog.Exitf("Account: %v", err)
	}
	if needSync {
		if err := o.Sync(ctx); err != nil {
			klog.Exitf("Sync: %v", err)
		}
		a, _, err = o.Account(ctx)
		if err != nil {
			klog.Exitf("Account: %v", err)
		}
	}

	ps := a.Properties[0]
	em := ps.ElectricityMeterPoints[0]

	now := time.Now().Add(-24 * time.Hour * 3).Truncate(30 * time.Minute)
	end := now.Add(8 * time.Hour)
	klog.Infof("Now: %v", now)
	klog.Infof("End: %v", end)
	//end := now.Add(15 * time.Minute)
	cons, err := o.Consumption(ctx, em.MPAN, em.Meters[0].SerialNumber, now, end)
	if err != nil {
		klog.Exitf("Consumption: %v", err)
	}
	klog.Infof("Cons: %+v", cons)

	if err := o.SyncTariff(ctx, "AGILE-24-04-03", "E-1R-AGILE-24-04-03-J", now, end); err != nil {
		klog.Exitf("SyncTariff: %v", err)
	}

	rates, err := o.TariffRates(ctx, em.ActiveAgreement(now).TariffCode, now, end)
	if err != nil {
		klog.Exitf("Tariff: %v", err)
	}
	klog.Infof("Tariff: %+v", rates)

	//cost, err := octonaut.TotalCost(ctx, cons, octonaut.FlatRate(0.25))
	cost, err := octonaut.TotalCost(ctx, cons, octonaut.Tariff(*rates))
	if err != nil {
		klog.Exitf("TotalCost: %v", err)
	}
	klog.Infof("TotalCost: £%.2f", cost/100.0)

}

func mustNewFromFlags(ctx context.Context) (func() error, *octonaut.Octonaut) {
	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		klog.Exitf("Failed to open DB (%q): %v", *dbPath, err)
	}

	u := *ep
	if !strings.HasSuffix(u, "/") {
		u += "/"
	}

	r, err := octonaut.New(ctx, *account, *key, u, db)
	if err != nil {
		klog.Exitf("New: %v", err)
	}

	return db.Close, r
}
