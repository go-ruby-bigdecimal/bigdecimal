// Copyright (c) the go-ruby-bigdecimal/bigdecimal authors
//
// SPDX-License-Identifier: BSD-3-Clause

package bigdecimal

import "math/big"

// roundTo rounds the value so its least-significant digit sits at 10**place,
// using mode. place > 0 rounds away whole digits (round(-1) etc.), place == 0
// rounds to an integer, place < 0 keeps |place| fractional digits. A value
// already coarser than place passes through unchanged. Specials pass through.
func (d *Decimal) roundTo(place int, mode RoundMode) *Decimal {
	if d.kind != finite {
		if d.kind == nan {
			return nanDecimal()
		}
		return infDecimal(d.sign)
	}
	if d.coef.Sign() == 0 {
		return newFiniteOwned(d.sign, new(big.Int), 0)
	}
	if d.exp >= place {
		return d.clone() // already at or above the requested granularity
	}
	drop := place - d.exp
	q, up := roundShift(d.coef, drop, mode, d.sign, d.intDropped(place))
	if up {
		q.Add(q, bigOne)
	}
	return newFiniteOwned(d.sign, q, place)
}

// intDropped reports whether ROUND_UP should treat the discarded remainder as
// non-zero when rounding to `place`. When rounding to a fractional or units place
// (place <= 0) the full discarded remainder counts, so the answer is true (this
// helper is consulted only after the zero-remainder case is handled). When
// rounding away whole digits (place >= 1) MRI ignores the purely-fractional tail:
// the answer is true only when a discarded *integer*-position digit (10**0 or
// above) is non-zero — so 0.99 rounded to tens does NOT round away from zero.
func (d *Decimal) intDropped(place int) bool {
	if place <= 0 {
		return true // every discarded digit counts at a fractional/units place
	}
	if d.exp >= 0 {
		return true // the whole discarded remainder is integer-valued
	}
	// Strip the fractional low digits of the discarded remainder; a non-zero
	// quotient means an integer-position digit was discarded.
	rem := new(big.Int).Mod(d.coef, pow10(place-d.exp))
	return new(big.Int).Quo(rem, pow10(-d.exp)).Sign() != 0
}

// Round returns the value rounded to n decimal places under mode
// (BigDecimal#round(n, mode)). n > 0 keeps n fractional digits, n == 0 rounds to
// an integer, n < 0 rounds away |n| whole digits.
func (d *Decimal) Round(n int, mode RoundMode) *Decimal {
	return d.roundTo(-n, mode)
}

// Floor returns the value rounded toward -Infinity to n places
// (BigDecimal#floor(n)).
func (d *Decimal) Floor(n int) *Decimal { return d.roundTo(-n, RoundFloor) }

// Ceil returns the value rounded toward +Infinity to n places
// (BigDecimal#ceil(n)).
func (d *Decimal) Ceil(n int) *Decimal { return d.roundTo(-n, RoundCeiling) }

// Truncate returns the value truncated toward zero to n places
// (BigDecimal#truncate(n)).
func (d *Decimal) Truncate(n int) *Decimal { return d.roundTo(-n, RoundDown) }

// Frac returns the fractional part (BigDecimal#frac): the value with its integer
// part removed, keeping the sign. A special returns itself.
func (d *Decimal) Frac() *Decimal {
	if d.kind != finite {
		return d.clone()
	}
	if d.exp >= 0 { // an integer value has no fraction
		return newFinite(d.sign, big.NewInt(0), 0)
	}
	intPart := d.Truncate(0)
	return d.Sub(intPart)
}

// Fix returns the integer part toward zero (BigDecimal#fix), as a Decimal.
func (d *Decimal) Fix() *Decimal {
	if d.kind != finite {
		return d.clone()
	}
	return d.Truncate(0)
}

// Cmp compares d and o, returning -1, 0 or 1 (BigDecimal#<=>). It returns -2 to
// signal "incomparable" when either operand is NaN (MRI's <=> yields nil there);
// callers map -2 to nil.
func (d *Decimal) Cmp(o *Decimal) int {
	if d.kind == nan || o.kind == nan {
		return -2
	}
	dv := d.orderKey()
	ov := o.orderKey()
	if dv != ov {
		if dv < ov {
			return -1
		}
		return 1
	}
	// Same broad class. Both ±∞ of equal sign compare equal.
	if d.kind == inf {
		return 0
	}
	return d.cmpFinite(o)
}

// orderKey buckets specials so infinities sort outside every finite value.
func (d *Decimal) orderKey() int {
	if d.kind == inf {
		return d.sign * 2 // +2 / -2
	}
	switch s := d.signValue(); {
	case s < 0:
		return -1
	case s > 0:
		return 1
	default:
		return 0
	}
}

// cmpFinite compares two finite values of the same broad sign bucket.
func (d *Decimal) cmpFinite(o *Decimal) int {
	if d.coef.Sign() == 0 && o.coef.Sign() == 0 {
		return 0
	}
	ca, cb, _ := alignedCoefs(d, o)
	a := d.applySign(ca)
	b := o.applySign(cb)
	return a.Cmp(b)
}

// signValue returns the arithmetic sign (-1/0/1) of a finite value, ignoring the
// stored sign of a zero. It is only reached from orderKey for a finite value
// (NaN short-circuits Cmp and ±∞ is handled before the call).
func (d *Decimal) signValue() int {
	if d.coef.Sign() == 0 {
		return 0
	}
	return d.sign
}

// Equal reports value equality (BigDecimal#==). NaN is unequal to everything,
// including itself.
func (d *Decimal) Equal(o *Decimal) bool {
	if d.kind == nan || o.kind == nan {
		return false
	}
	return d.Cmp(o) == 0
}

// Sign returns MRI's BigDecimal#sign code: 0 for NaN, 1/-1 for ±0, 2/-2 for a
// finite non-zero, 3/-3 for ±Infinity.
func (d *Decimal) Sign() int {
	switch d.kind {
	case nan:
		return 0
	case inf:
		if d.sign < 0 {
			return -3
		}
		return 3
	default:
		if d.coef.Sign() == 0 {
			return d.sign // ±1 for ±0
		}
		if d.sign < 0 {
			return -2
		}
		return 2
	}
}
