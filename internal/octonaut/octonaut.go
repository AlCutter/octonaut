package octonaut

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AlCutter/octonaut/internal/octopus"
	"k8s.io/klog/v2"
)

type Octonaut struct {
	c       *octopus.Client
	db      *sql.DB
	account octopus.Account
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

	ac, err := r.c.Account(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account: %v", err)
	}
	r.account = ac

	return r, nil
}

func initDB(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS Account(
			Number	string NOT NULL PRIMARY KEY
		);
		`); err != nil {
		return fmt.Errorf("failed to create Account table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS Property(
			ID			string NOT NULL,
			Account		string NOT NULL,
			MovedInAt	DATETIME NOT NULL,
			MovedOutAt	DATETIME,
			PRIMARY KEY (ID),
			FOREIGN KEY (Account) REFERENCES Account(Number)
		);
		`); err != nil {
		return fmt.Errorf("failed to create Property table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS Meter(
			Property	string NOT NULL,
			MPAN		string NOT NULL,
			Serial		string NOT NULL,
			PRIMARY KEY (Property, MPAN, Serial),
			FOREIGN KEY (Property) REFERENCES Property(ID)
		);
		`); err != nil {
		return fmt.Errorf("failed to create Meter table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS MeterRate(
			Meter		string NOT NULL,
			Rate		string NOT NULL,
			PRIMARY KEY (Meter, Rate),
			FOREIGN KEY (Meter) REFERENCES Meter(ID)
		);
		`); err != nil {
		return fmt.Errorf("failed to create MeterRate table: %v", err)
	}

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS Consumption(
			Account			string NOT NULL,
			MPAN			string NOT NULL,
			Meter			string NOT NULL,
			IntervalStart	DateTime NOT NULL,
			IntervalEnd		DateTime,
			kWh				REAL NOT NULL,
			PRIMARY KEY (Account, MPAN, Meter, IntervalStart));
		`); err != nil {
		return fmt.Errorf("create Consumption table failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS TariffRate(
			Code	string,
			At		DateTime,
			PerUnit REAL,
			PRIMARY KEY (Code, At));
		`); err != nil {
		return fmt.Errorf("create TariffRate table failed: %v", err)
	}
	return nil
}

func (o Octonaut) Sync(ctx context.Context) error {
	klog.V(1).Infof("Syncing %s", o.account.Number)
	a, err := o.Account(ctx)
	if err != nil {
		return err
	}
	if err := o.upsertAccount(ctx, a); err != nil {
		return err
	}
	o.account = a

	for _, p := range a.Properties {
		klog.V(1).Infof("  Syncing propery %d", p.ID)
		for _, em := range p.ElectricityMeterPoints {
			klog.V(1).Infof("    Syncing MPAN %s", em.MPAN)
			for _, m := range em.Meters {
				if m.SerialNumber != "" {
					klog.V(1).Infof("      Syncing Meter %s", m.SerialNumber)
					lastReading, err := o.consumptionMostRecent(ctx, em.MPAN, m.SerialNumber)
					if err != nil {
						klog.Warningf("Error reading local consumption date: %v", err)
						lastReading = p.MovedInAt
					}
					klog.V(1).Infof("        Syncing Consumption since %v", lastReading)
					c, err := o.c.Consumption(ctx, em.MPAN, m.SerialNumber, lastReading, time.Now())
					klog.V(1).Infof("        Got %d records", len(c.Results))
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
	tx, err := o.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO Account VALUES(?)`, a.Number); err != nil {
		return fmt.Errorf("insert/update account: %v", err)
	}

	for _, p := range a.Properties {
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO Property VALUES(?, ?, ?, ?)`, p.ID, a.Number, p.MovedInAt, p.MovedOutAt); err != nil {
			return fmt.Errorf("insert/update property: %v", err)
		}
		for _, em := range p.ElectricityMeterPoints {
			for _, m := range em.Meters {
				if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO Meter VALUES(?, ?, ?)`, p.ID, em.MPAN, m.SerialNumber); err != nil {
					return fmt.Errorf("insert/update Meter: %v", err)
				}
			}
		}
	}
	return tx.Commit()
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
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO Consumption VALUES(?, ?, ?, ?, ?, ?)`, o.account.Number, mpan, serial, cr.IntervalStart, cr.IntervalEnd, cr.Consumption); err != nil {
			return fmt.Errorf("insert/update account: %v", err)
		}
	}

	return tx.Commit()
}

func (o *Octonaut) consumptionMostRecent(ctx context.Context, mpan, serial string) (time.Time, error) {
	r := o.db.QueryRowContext(ctx, "SELECT MAX(IntervalStart) FROM Consumption WHERE Account = ? AND MPAN = ? AND Meter = ? ", o.account.Number, mpan, serial)
	var atStr string
	if err := r.Scan(&atStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("failed to scan latest Consumption.at: %v", err)
	}
	return time.Parse(time.RFC3339, atStr)

}

func (o *Octonaut) nextTariffRate(ctx context.Context, code string) (time.Time, error) {
	r := o.db.QueryRowContext(ctx, "SELECT MAX(At) FROM TariffRate WHERE Code = ?", code)
	var atStr string
	if err := r.Scan(&atStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("failed to scan latest TariffRate.at: %v", err)
	}
	return time.Parse(time.RFC3339, atStr)
}

func (o *Octonaut) Account(ctx context.Context) (octopus.Account, error) {
	return o.account, nil
}

func (o *Octonaut) Products(ctx context.Context, at *time.Time) (octopus.Products, error) {
	return o.c.Products(ctx, at)
}

func (o *Octonaut) TariffRates(ctx context.Context, product, fuel, tarrif, rate string, from, to time.Time) (octopus.TariffRate, error) {
	return o.c.TariffRates(ctx, product, fuel, tarrif, rate, from, to)
}
