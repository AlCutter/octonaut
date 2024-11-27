package octonaut

import (
	"time"
)

type TransferFunc func(ConsumptionInterval) ConsumptionInterval

func Apply(tf TransferFunc, c Consumption) Consumption {
	r := c
	r.Intervals = make([]ConsumptionInterval, 0, len(c.Intervals))
	for _, i := range c.Intervals {
		r.Intervals = append(r.Intervals, tf(i))
	}
	return r
}

type LoadShiftStats struct {
	Intervals []LoadShiftIntervalStats
}

func (l *LoadShiftStats) Headers() []string {
	return []string{"BatteryCharge", "BatteryDelta", "BatteryFull"}
}

func (l *LoadShiftStats) NumIntervals() int {
	return len(l.Intervals)
}

func (l *LoadShiftStats) Interval(i int) []any {
	d := l.Intervals[i]
	return []any{d.BatteryCharge, d.BatteryDelta, d.BatteryFull}
}

type LoadShiftIntervalStats struct {
	BatteryCharge float64
	BatteryDelta  float64
	BatteryFull   bool
}

func LoadShift(capacity float64, chargeRate float64, serviceLimit float64, startHour, endHour int) (TransferFunc, *LoadShiftStats) {
	charge := float64(0)
	stats := &LoadShiftStats{}

	tf := func(c ConsumptionInterval) ConsumptionInterval {
		hours := float64(c.End.Sub(c.Start)) / float64(time.Hour)
		inShift := c.Start.Hour() >= startHour && c.Start.Hour() <= endHour
		r := c
		batteryDelta := float64(0)
		if inShift {
			if charge < capacity {
				amt := chargeRate * hours
				space := capacity - charge
				if amt > space {
					amt = space
				}
				batteryDelta = amt
			}
		} else {
			fromBattery := charge
			if charge > r.Consumption {
				fromBattery = r.Consumption
			}
			batteryDelta = -fromBattery
		}
		charge += batteryDelta
		r.Consumption += batteryDelta
		stats.Intervals = append(stats.Intervals, LoadShiftIntervalStats{
			BatteryDelta:  batteryDelta,
			BatteryCharge: charge,
			BatteryFull:   charge == capacity,
		})

		return r
	}
	return tf, stats
}
