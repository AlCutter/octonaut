package octopus

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func accountPath(a string) string { return fmt.Sprintf("v1/accounts/%s/", a) }
func consumptionPath(mpan string, serial string, from, to time.Time, N int) string {
	return fmt.Sprintf("v1/electricity-meter-points/%s/meters/%s/consumption/?page_size=%d&period_from=%s&period_to=%s&order_by=period",
		mpan, serial, N, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339))
}
func tariffRatePath(product string, fuel string, tariff string, rate string, from, to time.Time, N int) string {
	return fmt.Sprintf("v1/products/%s/%s-tariffs/%s/%s/?page_size=%d&period_from=%s&period_to=%s", product, fuel, tariff, rate, N, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339))
}

type Account struct {
	Number     string     `json:"number"`
	Properties []Property `json:"properties"`
}

type Property struct {
	ID                     int                      `json:"id"`
	MovedInAt              time.Time                `json:"moved_in_at"`
	MovedOutAt             *time.Time               `json:"moved_out_at"`
	AddressLine1           string                   `json:"address_line_1"`
	AddressLine2           string                   `json:"address_line_2"`
	AddressLine3           string                   `json:"address_line_3"`
	Town                   string                   `json:"town"`
	County                 string                   `json:"county"`
	Postcode               string                   `json:"postcode"`
	ElectricityMeterPoints []ElectricityMeterPoints `json:"electricity_meter_points"`
	GasMeterPoints         []GasMeterPoints         `json:"gas_meter_points"`
}

type ElectricityMeterPoints struct {
	MPAN                string      `json:"mpan"`
	ProfileClass        int         `json:"profile_class"`
	ConsumptionStandard int         `json:"consumption_standard"`
	Meters              []Meter     `json:"meters"`
	Agreements          []Agreement `json:"agreements"`
}

func (em ElectricityMeterPoints) ActiveMeters() []Meter {
	r := []Meter{}
	for i := range em.Meters {
		if em.Meters[i].SerialNumber != "" {
			r = append(r, em.Meters[i])
		}
	}
	return r
}

func (em ElectricityMeterPoints) ActiveAgreement(at time.Time) *Agreement {
	for _, a := range em.Agreements {
		if at.Before(a.ValidFrom) {
			continue
		}
		if a.ValidTo != nil && at.After(*a.ValidTo) {
			continue
		}
		return &a
	}
	return nil
}

type Meter struct {
	SerialNumber string     `json:"serial_number"`
	Registers    []Register `json:"registers"`
}

type Register struct {
	Identifier           string `json:"identifier"`
	Rate                 string `json:"rate"`
	IsSettlementRegister bool   `json:"is_settlement_register"`
}

type Agreement struct {
	TariffCode string     `json:"tariff_code"`
	ValidFrom  time.Time  `json:"valid_from"`
	ValidTo    *time.Time `json:"valid_to"`
}

type GasMeterPoints struct {
	MPRN                string      `json:"mprn"`
	ProfileClass        int         `json:"profile_class"`
	ConsumptionStandard int         `json:"consumption_standard"`
	Meters              []Meter     `json:"meters"`
	Agreements          []Agreement `json:"agreements"`
}

type Consumption struct {
	Count    int    `json:"count"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Results  []struct {
		Consumption   float64 `json:"consumption"`
		IntervalStart string  `json:"interval_start"`
		IntervalEnd   string  `json:"interval_end"`
	} `json:"results"`
}

type TariffRate struct {
	Count    int    `json:"count"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Results  []struct {
		ValueExcVat float64   `json:"value_exc_vat"`
		ValueIncVat float64   `json:"value_inc_vat"`
		ValidFrom   time.Time `json:"valid_from"`
		ValidTo     time.Time `json:"valid_to"`
	} `json:"results"`
}

type Client struct {
	EndPoint  string
	AccountID string
	Key       string
}

func (c *Client) Account(ctx context.Context) (Account, error) {
	r := Account{}
	return r, c.get(ctx, accountPath(c.AccountID), &r)
}

func (c *Client) Consumption(ctx context.Context, mpan string, serial string, from time.Time, to time.Time) (Consumption, error) {
	N := 2000

	r := Consumption{}
	req := consumptionPath(mpan, serial, from, to, N)
	for req != "" {
		page := Consumption{}
		if err := c.get(ctx, req, &r); err != nil {
			return Consumption{}, err
		}
		r.Count = page.Count
		r.Results = append(r.Results, page.Results...)
		req = page.Next
	}

	return r, nil
}

func (c *Client) TariffRates(ctx context.Context, prod, fuel, tariff, rate string, from time.Time, to time.Time) (TariffRate, error) {
	N := 2000

	r := TariffRate{}
	req := tariffRatePath(prod, fuel, tariff, rate, from, to, N)
	for req != "" {
		page := TariffRate{}
		if err := c.get(ctx, req, &r); err != nil {
			return TariffRate{}, err
		}
		r.Count = page.Count
		r.Results = append(r.Results, page.Results...)
		req = page.Next
	}

	return r, nil
}

func (c *Client) get(ctx context.Context, p string, out any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.EndPoint+p, nil)
	if err != nil {
		return fmt.Errorf("NewRequestWithContext: %v", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(c.Key))))
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Do: %v", err)
	}
	if rsp.StatusCode != 200 {
		return fmt.Errorf("Do(%q): unexpected status %s", p, rsp.Status)
	}
	defer rsp.Body.Close()
	b, err := io.ReadAll(rsp.Body)
	if err != nil {
		return fmt.Errorf("Read(%s): %v", p, err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("Unmarshal(%s): %v", p, err)
	}
	return nil
}
