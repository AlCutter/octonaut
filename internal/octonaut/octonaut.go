package octonaut

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AlCutter/octonaut/internal/octopus"
	"k8s.io/klog/v2"
)

type Octonaut struct {
	c  *octopus.Client
	db *sql.DB

	account *octopus.Account
}

func New(ctx context.Context, a, k, ep string, db *sql.DB) (*Octonaut, error) {
	if !strings.HasSuffix(ep, "/") {
		ep += "/"
	}

	r := &Octonaut{
		c: &octopus.Client{
			EndPoint:  ep,
			AccountID: a,
			Key:       k,
		},
		db: db,
	}

	if err := initDB(ctx, r.db); err != nil {
		return nil, fmt.Errorf("initDB: %v", err)
	}

	/*
		account, err := r.Account(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch account: %v", err)
		}
		r.account = account
	*/
	return r, nil
}

func initDB(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS Account(
			Number	string NOT NULL PRIMARY KEY,
			JSON	string
		);
		`); err != nil {
		return fmt.Errorf("failed to create Account table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS Consumption(
			Account			string NOT NULL,
			MPAN			string NOT NULL,
			Meter			string NOT NULL,
			IntervalStart	Timestamp NOT NULL,
			IntervalEnd		Timestamp,
			kWh				REAL NOT NULL,
			PRIMARY KEY (Account, MPAN, Meter, IntervalStart));
		`); err != nil {
		return fmt.Errorf("create Consumption table failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS TariffRate(
			Code			string NOT NULL,
			ValidFrom		Timestamp NOT NULL,
			ValidTo			Timestamp,
			UnitCostIncVAT	REAL NOT NULL,
			PRIMARY KEY (Code, ValidFrom ASC));
		`); err != nil {
		return fmt.Errorf("create TariffRate table failed: %v", err)
	}
	return nil
}

func (o Octonaut) Sync(ctx context.Context) error {
	klog.Infof("Syncing %s", o.c.AccountID)
	a, err := o.c.Account(ctx)
	if err != nil {
		return err
	}
	if err := o.upsertAccount(ctx, a); err != nil {
		return err
	}
	o.account = &a

	for _, p := range a.Properties {
		klog.Infof(" + Syncing property %d", p.ID)
		for _, em := range p.ElectricityMeterPoints {
			klog.Infof(" | + Syncing MPAN %s", em.MPAN)
			for _, m := range em.Meters {
				if m.SerialNumber != "" {
					klog.Infof(" | | + Syncing Meter %s", m.SerialNumber)
					lastReading, err := o.consumptionMostRecent(ctx, em.MPAN, m.SerialNumber)
					if err != nil {
						klog.Warningf("Error reading local consumption date: %v", err)
						lastReading = p.MovedInAt
					}
					klog.Infof(" | | | + Syncing Consumption since %v", lastReading)
					c, err := o.c.Consumption(ctx, em.MPAN, m.SerialNumber, lastReading, time.Now())
					klog.Infof(" | | | | Got %d records", len(c.Results))
					if err := o.insertConsumption(ctx, em.MPAN, m.SerialNumber, c); err != nil {
						klog.Warningf("Failed to store consumption data: %v", err)
					}
				}
			}
		}
	}

	return nil
}

func (o *Octonaut) upsertAccount(ctx context.Context, a octopus.Account) error {
	j, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("marshal: %v", err)
	}

	if _, err := o.db.ExecContext(ctx, `INSERT OR REPLACE INTO Account VALUES(?, ?)`, a.Number, j); err != nil {
		return fmt.Errorf("insert/update account: %v", err)
	}
	return nil
}

func (o *Octonaut) insertConsumption(ctx context.Context, mpan, serial string, c octopus.Consumption) error {
	tx, err := o.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()

	for _, cr := range c.Results {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO Consumption VALUES(?, ?, ?, ?, ?, ?)`,
			o.account.Number,
			mpan,
			serial,
			cr.IntervalStart.Unix(), cr.IntervalEnd.Unix(), cr.Consumption); err != nil {
			return fmt.Errorf("insert/update account: %v", err)
		}
	}

	return tx.Commit()
}

func (o *Octonaut) consumptionMostRecent(ctx context.Context, mpan, serial string) (time.Time, error) {
	r := o.db.QueryRowContext(ctx, "SELECT datetime(MAX(IntervalStart)) FROM Consumption WHERE Account = ? AND MPAN = ? AND Meter = ? ", o.account.Number, mpan, serial)
	var start sql.NullTime
	if err := r.Scan(&start); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("failed to scan latest Consumption.at: %v", err)
	}
	return start.Time, nil
}

func (o *Octonaut) SyncTariff(ctx context.Context, product, tariffCode string, from time.Time, to time.Time) error {
	t, err := o.c.TariffRates(ctx, product, "electricity", tariffCode, "standard-unit-rates", from, to)
	if err != nil {
		return fmt.Errorf("TariffRates: %v", err)
	}

	if err := o.upsertTariff(ctx, tariffCode, t); err != nil {
		return fmt.Errorf("Upsert: %v", err)
	}

	return nil
}

func (o *Octonaut) upsertTariff(ctx context.Context, tariffCode string, t octopus.TariffRate) error {
	tx, err := o.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()

	for _, r := range t.Results {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO TariffRate VALUES(?, ?, ?, ?)`,
			tariffCode, r.ValidFrom.Unix(), r.ValidTo.Unix(), r.ValueIncVat); err != nil {
			return fmt.Errorf("insert/update tariffrate: %v", err)
		}
	}

	return tx.Commit()
}

func (o *Octonaut) Account(ctx context.Context) (*octopus.Account, bool, error) {
	r := o.db.QueryRowContext(ctx, "SELECT JSON from Account WHERE Number = ?", o.c.AccountID)
	var j []byte
	if err := r.Scan(&j); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("Scan: %v", err)
	}
	a := &octopus.Account{}
	if err := json.Unmarshal(j, &a); err != nil {
		return nil, false, fmt.Errorf("Unmarshal: %v", err)
	}
	// Now check for invalid meters and remove them
	for pi, p := range a.Properties {
		fem := []octopus.ElectricityMeterPoint{}
		for ei, em := range p.ElectricityMeterPoints {
			if em.MPAN == "" {
				continue
			}
			fms := []octopus.Meter{}
			for _, m := range em.Meters {
				if m.SerialNumber == "" {
					continue
				}
				fms = append(fms, m)
			}
			if len(fms) > 0 {
				a.Properties[pi].ElectricityMeterPoints[ei].Meters = fms
				fem = append(fem, em)
			}
		}
	}
	return a, false, nil
}

func (o *Octonaut) Products(ctx context.Context, at *time.Time) (octopus.Products, error) {
	return o.c.Products(ctx, at)
}

func (o *Octonaut) TariffRates(ctx context.Context, tariffCode string, from, to time.Time) (*octopus.TariffRate, error) {
	r := octopus.TariffRate{}
	q := `
		SELECT ValidFrom, ValidTo, UnitCostIncVAT FROM TariffRate WHERE Code = $code AND ValidFrom <= $from AND ValidTo >= $from
		UNION
		SELECT ValidFrom, ValidTo, UnitCostIncVAT FROM TariffRate WHERE Code = $code AND ValidFrom > $from AND ValidFrom <= $to
		ORDER BY ValidFrom ASC`
	args := []any{
		sql.Named("code", tariffCode),
		sql.Named("from", from.Unix()),
		sql.Named("to", to.Unix())}
	rows, err := o.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("QueryContext: %v", err)
	}
	var last *time.Time
	for rows.Next() {
		var start time.Time
		var end sql.NullTime
		var k float64
		if err := rows.Scan(&start, &end, &k); err != nil {
			return nil, fmt.Errorf("Scan: %v", err)
		}

		if last != nil && !last.Equal(start) {
			return nil, fmt.Errorf("missing data between %v and %v", last, start)
		}
		r.Results = append(r.Results, octopus.RateInterval{
			ValidFrom:   start,
			ValidTo:     end.Time,
			ValueIncVat: k,
		})
		from = start
		last = &(end.Time)
	}
	if len(r.Results) == 0 {
		return nil, errors.New("no data")
	}
	klog.Infof("%d TariffRates", len(r.Results))
	return &r, nil
}

func (o *Octonaut) Consumption(ctx context.Context, mpan, meter string, from time.Time, to time.Time) (*Consumption, error) {
	r := Consumption{}
	q := `
		SELECT IntervalStart, IntervalEnd, kWh FROM Consumption WHERE Account = $account AND MPAN = $mpan AND Meter = $meter AND IntervalStart <= $from AND IntervalEnd > $from
		UNION
		SELECT IntervalStart, IntervalEnd, kWh FROM Consumption WHERE Account = $account AND MPAN = $mpan AND Meter = $meter AND IntervalStart > $from AND IntervalStart <= $to
		ORDER BY IntervalStart ASC`
	args := []any{
		sql.Named("account", o.c.AccountID),
		sql.Named("mpan", mpan),
		sql.Named("meter", meter),
		sql.Named("from", from.Unix()),
		sql.Named("to", to.Unix())}
	rows, err := o.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("QueryContext: %v", err)
	}
	var last *time.Time
	for rows.Next() {
		var start time.Time
		var end sql.NullTime
		var k float64
		if err := rows.Scan(&start, &end, &k); err != nil {
			return nil, fmt.Errorf("Scan: %v", err)
		}
		if last != nil && !last.Equal(start) {
			klog.Warningf("Missing data between %v and %v, inserting zero usage intervals", last, start)
			for last.Before(start) {
				e := last.Add(30 * time.Minute)
				r.Intervals = append(r.Intervals, ConsumptionInterval{
					Start:       *last,
					End:         e,
					Consumption: 0,
				})
				last = &e
			}
		}
		r.Intervals = append(r.Intervals, ConsumptionInterval{
			Start:       start,
			End:         end.Time,
			Consumption: k,
		})
		last = &(end.Time)
	}
	if len(r.Intervals) == 0 {
		return nil, errors.New("no data")
	}
	klog.Infof("%d ConsumptionIntervals", len(r.Intervals))
	return &r, nil
}
