package octonaut

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/AlCutter/octonaut/internal/octopus"
)

type Octonaut struct {
	c  *octopus.Client
	db *sql.DB
}

func New(a, k, ep string, db *sql.DB) *Octonaut {
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

	return r
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
