package domain

// Currency is an ISO 4217 code such as "COP". Amounts are always int64 counts of
// the currency's minor unit (cents for COP or USD, whole yen for JPY), never floats.
type Currency string

// Decimals returns how many decimal places the minor unit of c has (0 for JPY,
// 2 for USD, 3 for KWD), and false if c is not a current ISO 4217 currency.
func (c Currency) Decimals() (int, bool) {
	d, ok := decimals[c]
	return d, ok
}
