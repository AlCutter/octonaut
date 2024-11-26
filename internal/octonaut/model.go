package octonaut

import (
	"time"

	"k8s.io/klog/v2"
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

func LoadShift(capacity float64, chargeRate float64, serviceLimit float64, startHour, endHour int) TransferFunc {
	charge := float64(0)
	return func(c ConsumptionInterval) ConsumptionInterval {
		hours := float64(c.End.Sub(c.Start)) / float64(time.Hour)
		inShift := c.Start.Hour() >= startHour && c.Start.Hour() <= endHour
		r := c
		if inShift {
			if charge < capacity {
				amt := chargeRate * hours
				space := capacity - charge
				if amt > space {
					amt = space
				}
				charge += amt
				r := c
				r.Consumption += amt
				klog.Infof("%v charge %.2f kWh [%-.2f]", c.Start.Format(time.Stamp), charge, amt)
				return r
			}
			klog.Infof("%v charge %.2f kWh [full]", c.Start.Format(time.Stamp), charge)
			return r
		}
		fromBattery := charge
		if charge > r.Consumption {
			fromBattery = r.Consumption
		}
		charge -= fromBattery
		r.Consumption -= fromBattery
		klog.Infof("%v charge %.2f kWh [%-.2f]", c.Start.Format(time.Stamp), charge, -fromBattery)

		return r
	}
}
