package octopus

import (
	"testing"
)

func TestParseTariffCode(t *testing.T) {
	for _, test := range []struct {
		tariff                                         string
		wantFuel, wantRegisters, wantProduct, wantArea string
		wantErr                                        bool
	}{
		{
			tariff:        "E-1R-GO-VAR-22-10-14-J",
			wantFuel:      "E",
			wantRegisters: "1R",
			wantProduct:   "GO-VAR-22-10-14",
			wantArea:      "J",
		}, {
			tariff:        "A-BOB-TARIFF-BANANA",
			wantFuel:      "A",
			wantRegisters: "BOB",
			wantProduct:   "TARIFF",
			wantArea:      "BANANA",
		}, {
			tariff:  "A-BOB-TARIFF",
			wantErr: true,
		},
	} {
		t.Run(test.tariff, func(t *testing.T) {
			f, r, p, a, err := ParseTariffCode(test.tariff)
			if gotErr := err != nil; gotErr {
				if gotErr != test.wantErr {
					t.Fatalf("got err %v, wantErr %v", err, test.wantErr)
				}
				if test.wantErr {
					return
				}
			}
			if f != test.wantFuel {
				t.Errorf("Got fuel %s, want %s", f, test.wantFuel)
			}
			if r != test.wantRegisters {
				t.Errorf("Got registers %s, want %s", r, test.wantRegisters)
			}
			if p != test.wantProduct {
				t.Errorf("Got product %s, want %s", p, test.wantProduct)
			}
			if a != test.wantArea {
				t.Errorf("Got area %s, want %s", a, test.wantArea)
			}
		})
	}
}
