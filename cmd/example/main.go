package main

import (
	"context"
	"flag"
	"log"
	"strings"
	"time"

	"github.com/AlCutter/octonaut/internal/octopus"
)

var (
	ep      = flag.String("endpoint", "https://api.octopus.energy/", "Base URL of the Octopus API.")
	account = flag.String("account", "", "Octopus Account e.g. A-123456.")
	key     = flag.String("key", "", "Octopus API key.")
	mpan    = flag.String("mpan", "", "Unique ID for an electricity meter.")
)

func main() {
	flag.Parse()
	u := *ep
	if !strings.HasSuffix(u, "/") {
		u += "/"
	}

	c := octopus.Client{
		EndPoint:  u,
		AccountID: *account,
		Key:       *key,
	}

	ctx := context.Background()
	a, err := c.Account(ctx)
	if err != nil {
		log.Fatalf("Account: %v", err)
	}
	log.Printf("Account: %+v", a)

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
		log.Fatal("Couldn't find electricity meterpoint with active meters")
	}

	log.Printf("Agreement: %+v", agreement)

	products, err := c.Products(ctx, &agreement.ValidFrom)
	if err != nil {
		log.Fatalf("Products: %v", err)
	}
	tariffProduct := products.FindByTariff(agreement.TariffCode)
	if tariffProduct == nil {
		log.Fatalf("Couldn't find product for code %q", agreement.TariffCode)
	}
	log.Printf("Tariff Product: %+v", tariffProduct)
	/*
		cons, err := c.Consumption(ctx, elecMeter.MPAN, elecMeter.Meters[0].SerialNumber, now.Add(-30*24*time.Hour), now)
		if err != nil {
			log.Fatalf("Consumption: %v", err)
		}
		log.Printf("Cons: %+v", cons)
	*/

	tariff, err := c.TariffRates(ctx, tariffProduct.Code, "electricity", agreement.TariffCode, "standard-unit-rates", now.Add(-30*24*time.Hour), now)
	if err != nil {
		log.Fatalf("Tariff: %v", err)
	}
	log.Printf("Tariff: %+v", tariff)
}
