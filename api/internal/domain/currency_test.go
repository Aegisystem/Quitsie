package domain_test

import (
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
)

func TestCurrencyDecimals(t *testing.T) {
	tests := []struct {
		currency domain.Currency
		decimals int
	}{
		{"JPY", 0},
		{"CLP", 0},
		{"USD", 2},
		{"COP", 2},
		{"EUR", 2},
		{"BHD", 3},
		{"KWD", 3},
		{"CLF", 4},
		{"XCG", 2},
	}
	for _, tt := range tests {
		got, ok := tt.currency.Decimals()
		if !ok || got != tt.decimals {
			t.Errorf("%s.Decimals() = %d, %t; want %d, true", tt.currency, got, ok, tt.decimals)
		}
	}
}

func TestCurrencyDecimalsRejectsCodesThatAreNotCurrentCurrencies(t *testing.T) {
	for _, c := range []domain.Currency{
		"",
		"usd", // codes are upper case
		"ANG", // withdrawn, replaced by XCG
		"HRK", // withdrawn, Croatia adopted the euro
		"XAU", // gold has no minor unit
		"XXX", // "no currency"
		"ABC",
	} {
		if got, ok := c.Decimals(); ok {
			t.Errorf("%q.Decimals() = %d, true; want false", c, got)
		}
	}
}
