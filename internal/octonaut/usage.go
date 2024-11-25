package octonaut

import (
	"context"
	"fmt"
	"time"

	"github.com/AlCutter/octonaut/internal/octopus"
	"k8s.io/klog/v2"
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
		klog.Infof("T: %v -> %v", from, to)
		if i >= len(t.Results) {
			return 0, fmt.Errorf("no more tariff entries")
		}
		for i < len(t.Results) {
			switch {
			case to.Before(t.Results[i].ValidFrom):
				return 0, fmt.Errorf("interval (%v -> %v) is before current rate interval (%v -> %v)", from, to, t.Results[i].ValidFrom, t.Results[i].ValidTo)
			case !t.Results[i].ValidFrom.After(from) && !t.Results[i].ValidTo.After(to):
				return t.Results[i].ValueIncVat * kWh, nil
			default:
				i++
				continue
			}
		}
		return 0, fmt.Errorf("no more tariff entries")
	}
}

func TotalCost(ctx context.Context, cons *Consumption, c CostFn) (float64, error) {
	total := float64(0)
	for _, u := range cons.Intervals {
		pence, err := c(ctx, u.Start, u.End, u.Consumption)
		if err != nil {
			return 0, fmt.Errorf("CostFN: %v", err)
		}
		total += pence
	}
	return total, nil
}
