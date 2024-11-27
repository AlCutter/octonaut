package octonaut

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
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

type RateFn func(ctx context.Context, start, end time.Time) (float64, error)

func FlatRate(kWhCost float64) RateFn {
	return func(_ context.Context, _, _ time.Time) (float64, error) {
		return kWhCost, nil
	}
}

func Tariff(t octopus.TariffRate) RateFn {
	i := 0
	return func(_ context.Context, from, to time.Time) (float64, error) {
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
				v := t.Results[i].ValueIncVat
				return v, nil
			default:
				return 0, fmt.Errorf("What happened? from %d to %d,  rate valid from %d to %d", fromU, toU, rFromU, rToU)
			}
		}
		return 0, fmt.Errorf("no more tariff entries, but need %d -> %d", fromU, toU)
	}
}

type Cost struct {
	TotalCost        float64
	TotalConsumption float64
	IntervalCosts    []ConsumptionIntervalCost
}

type IntervalStat interface {
	Headers() []string
	NumIntervals() int
	Interval(i int) []any
}

func csvify(as []any) []byte {
	r := [][]byte{}
	for _, a := range as {
		r = append(r, []byte(fmt.Sprintf("%v", a)))
	}
	return bytes.Join(r, []byte{','})

}

func (c *Cost) ToCSV(w io.Writer, stats ...IntervalStat) error {
	headers := []string{"Start", "End", "Consumption", "Rate", "Cost"}
	l := len(c.IntervalCosts)
	for _, s := range stats {
		headers = append(headers, s.Headers()...)
		if sl := s.NumIntervals(); sl != l {
			return fmt.Errorf("got %d cost intervals, but stats (%T) has %d intervals", l, s, sl)
		}
	}
	if _, err := w.Write(append([]byte(strings.Join(headers, ",")), '\n')); err != nil {
		return fmt.Errorf("writing headers: %v", err)
	}
	for i, ci := range c.IntervalCosts {
		vs := []any{ci.Start.Format(time.RFC3339), ci.End.Format(time.RFC3339), ci.Consumption, ci.Rate, ci.Cost}
		for _, s := range stats {
			vs = append(vs, s.Interval(i)...)
		}
		if _, err := w.Write(append(csvify(vs), '\n')); err != nil {
			return fmt.Errorf("writing headers: %v", err)
		}
	}

	return nil
}

type ConsumptionIntervalCost struct {
	ConsumptionInterval
	Rate float64
	Cost float64
}

func TotalCost(ctx context.Context, cons Consumption, c RateFn) (*Cost, error) {
	r := Cost{}
	for _, u := range cons.Intervals {
		rate, err := c(ctx, u.Start, u.End)
		if err != nil {
			return nil, fmt.Errorf("CostFN: %v", err)
		}
		pence := rate * u.Consumption
		r.TotalCost += pence
		r.TotalConsumption += u.Consumption
		r.IntervalCosts = append(r.IntervalCosts, ConsumptionIntervalCost{
			ConsumptionInterval: u,
			Cost:                pence,
			Rate:                rate,
		})
	}
	return &r, nil
}
