package octonaut

import (
	"context"
	"fmt"
	"time"

	"github.com/AlCutter/octonaut/internal/octopus"
)

type Consumption struct {
	Intervals []ConsumptionInterval
}

type ConsumptionInterval struct {
	Start       time.Time
	End         time.Time
	Consumption float64
}

type CostFn func(ctx context.Context, start, end time.Time, kWh float64) (float64, error)

func FlatRate(kWhCost float64) CostFn {
	return func(_ context.Context, _, _ time.Time, kWh float64) (float64, error) {
		return kWhCost * kWh, nil
	}
}

func Tariff(t octopus.TariffRate) CostFn {
	i := 0
	return func(_ context.Context, from, to time.Time, kWh float64) (float64, error) {
		fromU, toU := from.Unix(), to.Unix()
		if i >= len(t.Results) {
			return 0, fmt.Errorf("no more tariff entries")
		}
		for i < len(t.Results) {
			rFromU, rToU := t.Results[i].ValidFrom.Unix(), t.Results[i].ValidTo.Unix()
			switch {
			case toU < rFromU:
				return 0, fmt.Errorf("interval [%d] -> [%d] is before current rate interval [%d] ->  [%d]", fromU, toU, rFromU, rToU)
			case fromU >= rToU:
				i++
				continue
			case fromU >= rFromU && toU <= rToU:
				v := t.Results[i].ValueIncVat * kWh
				return v, nil
			default:
				return 0, fmt.Errorf("What happened? from %d to %d,  rate valid from %d to %d", fromU, toU, rFromU, rToU)
			}
		}
		return 0, fmt.Errorf("no more tariff entries, but need %d -> %d", fromU, toU)
	}
}

func TotalCost(ctx context.Context, cons *Consumption, c CostFn) (float64, float64, error) {
	totalCost := float64(0)
	totalConsumption := float64(0)
	for _, u := range cons.Intervals {
		pence, err := c(ctx, u.Start, u.End, u.Consumption)
		if err != nil {
			return 0, 0, fmt.Errorf("CostFN: %v", err)
		}
		totalCost += pence
		totalConsumption += u.Consumption
	}
	return totalCost, totalConsumption, nil
}
