package domain

import "math/bits"

// sum adds up int64 amounts exactly, in 128 bits. No number of records a
// ledger can hold makes it overflow, so a check against MaxBalance cannot be
// fooled by an int64 that wrapped around: 18,447 Expenses of MaxAmount add up
// to 1.8 × 10^19, which an int64 wraps to 2.6 × 10^14.
type sum struct {
	hi int64
	lo uint64
}

func (s *sum) add(x int64) {
	var carry uint64
	s.lo, carry = bits.Add64(s.lo, uint64(x), 0)
	// x>>63 is the high half of x: -1 when it is negative, 0 otherwise.
	s.hi += x>>63 + int64(carry)
}

func (s *sum) sub(x int64) {
	var borrow uint64
	s.lo, borrow = bits.Sub64(s.lo, uint64(x), 0)
	s.hi -= x>>63 + int64(borrow)
}

func (s *sum) addSum(t sum) {
	var carry uint64
	s.lo, carry = bits.Add64(s.lo, t.lo, 0)
	s.hi += t.hi + int64(carry)
}

// within reports whether the sum is between -limit and limit, for limit >= 0.
func (s sum) within(limit int64) bool {
	v := int64(s.lo)
	return s.hi == v>>63 && -limit <= v && v <= limit
}

// int64 is the sum if it fits in an int64; otherwise, its low 64 bits, which
// is what adding up in an int64 would have wrapped around to.
func (s sum) int64() int64 {
	return int64(s.lo)
}
