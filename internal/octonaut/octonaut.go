package octonaut

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AlCutter/octonaut/internal/octopus"
)

type Octonaut struct {
	c  *octopus.Client
	db *sql.DB
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

	return r, initDB(ctx, r.db)
}

func initDB(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS Account(
			Number string NOT NULL PRIMARY KEY
		);
		`); err != nil {
		return fmt.Errorf("failed to create Account table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS Property(
			ID string NOT NULL,
			Account string NOT NULL,
			MovedInAt DATETIME NOT NULL,
			MovedOutAt DATETIME,
			PRIMARY KEY (ID),
			FOREIGN KEY (Account) REFERENCES Account(Number)
		);
		`); err != nil {
		return fmt.Errorf("failed to create Property table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS Meter(
			Property string NOT NULL,
			MPAN string NOT NULL,
			Serial string NOT NULL,
			PRIMARY KEY (Property, MPAN, Serial),
			FOREIGN KEY (Property) REFERENCES Property(ID)
		);
		`); err != nil {
		return fmt.Errorf("failed to create Meter table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS MeterRate(
			Meter string NOT NULL,
			Rate string NOT NULL,
			PRIMARY KEY (Meter, Rate),
			FOREIGN KEY (Meter) REFERENCES Meter(ID)
		);
		`); err != nil {
		return fmt.Errorf("failed to create MeterRate table: %v", err)
	}

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS Consumption(
			Account string,
			MPAN	string,
			Meter	string,
			At		DateTime,
			Seconds	LONG INT,
			kWh		REAL,
			PRIMARY KEY (Account, MPAN, Meter, At));
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
	a, err := o.Account(ctx)
	if err != nil {
		return err
	}
	if err := o.upsertAccount(ctx, a); err != nil {
		return err
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

func (o *Octonaut) nextTariffRate(ctx context.Context, code string) (*time.Time, error) {
	r := o.db.QueryRowContext(ctx, "SELECT MAX(At) FROM TariffRate WHERE Code == ?", code)
	var at time.Time
	if err := r.Scan(&at); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &time.Time{}, nil
		}
		return nil, fmt.Errorf("failed to scan latest tariffrate.at: %v", err)
	}
	return &at, nil
}

func (o *Octonaut) Account(ctx context.Context) (octopus.Account, error) {
	return o.c.Account(ctx)
}

func (o *Octonaut) Products(ctx context.Context, at *time.Time) (octopus.Products, error) {
	return o.c.Products(ctx, at)
}

func (o *Octonaut) TariffRates(ctx context.Context, product, fuel, tarrif, rate string, from, to time.Time) (octopus.TariffRate, error) {
	return o.c.TariffRates(ctx, product, fuel, tarrif, rate, from, to)
}
