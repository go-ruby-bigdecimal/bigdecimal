// Copyright (c) the go-ruby-bigdecimal/bigdecimal authors
//
// SPDX-License-Identifier: BSD-3-Clause

package bigdecimal

import (
	"math"
	"math/big"
	"strconv"
)

// Int returns the value truncated toward zero as a *big.Int (BigDecimal#to_i).
// A NaN or ±Infinity returns nil (MRI raises; the caller decides).
func (d *Decimal) Int() *big.Int {
	if d.kind != finite {
		return nil
	}
	// Truncate(0) drops the fraction, so t is an integer with t.exp >= 0.
	t := d.Truncate(0)
	v := new(big.Int).Mul(t.coef, pow10(t.exp))
	if t.sign < 0 {
		v.Neg(v)
	}
	return v
}

// Float64 returns the nearest float64 (BigDecimal#to_f). NaN/±Infinity map to the
// matching IEEE value.
func (d *Decimal) Float64() float64 {
	switch d.kind {
	case nan:
		return math.NaN()
	case inf:
		return math.Inf(d.sign)
	}
	f, _ := strconv.ParseFloat(d.ToS("F"), 64)
	return f
}

// Rat returns the exact value as a *big.Rat (BigDecimal#to_r). A non-finite value
// returns nil.
func (d *Decimal) Rat() *big.Rat {
	if d.kind != finite {
		return nil
	}
	num := new(big.Int).Set(d.coef)
	den := big.NewInt(1)
	if d.exp >= 0 {
		num.Mul(num, pow10(d.exp))
	} else {
		den = pow10(-d.exp)
	}
	if d.sign < 0 {
		num.Neg(num)
	}
	return new(big.Rat).SetFrac(num, den)
}

// Exponent returns MRI's BigDecimal#exponent: the power of ten n such that the
// value equals 0.<digits> * 10**n. A zero or non-finite value returns 0.
func (d *Decimal) Exponent() int {
	if d.kind != finite {
		return 0
	}
	return d.pointExp()
}

// Precision returns the number of significant decimal digits (MRI 4.0's
// BigDecimal#precision). A zero returns 0; a non-finite value returns 0.
func (d *Decimal) Precision() int {
	if d.kind != finite || d.coef.Sign() == 0 {
		return 0
	}
	return d.numDigits()
}

// SplitParts implements BigDecimal#split: it returns the sign (1/-1, or 0 for
// NaN), the significant-digit string, the base (always 10) and the point
// exponent, so the value is sign * ("0." + digits).to_f * 10**exp. For a special
// the digit string is "NaN" / "Infinity" and exp is 0 (matching MRI).
func (d *Decimal) SplitParts() (sign int, digits string, base, exp int) {
	switch d.kind {
	case nan:
		return 0, "NaN", 10, 0
	case inf:
		return d.sign, "Infinity", 10, 0
	}
	if d.coef.Sign() == 0 {
		return d.sign, "0", 10, 0
	}
	return d.sign, d.coef.Text(10), 10, d.pointExp()
}
