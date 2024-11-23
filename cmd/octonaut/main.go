package main

import (
	"context"
	"database/sql"
	"flag"
	"strings"

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

	if err := o.Sync(ctx); err != nil {
		klog.Exitf("Sync: %v", err)
	}

	/*
		cons, err := c.Consumption(ctx, elecMeter.MPAN, elecMeter.Meters[0].SerialNumber, now.Add(-30*24*time.Hour), now)
		if err != nil {
			klog.Exitf("Consumption: %v", err)
		}
		klog.Infof("Cons: %+v", cons)
	*/

	/*
		_, err = o.TariffRates(ctx, tariffProduct.Code, "electricity", agreement.TariffCode, "standard-unit-rates", now.Add(-30*24*time.Hour), now)
		if err != nil {
			klog.Exitf("Tariff: %v", err)
		}
	*/
	//	klog.Infof("Tariff: %+v", tariff)
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
