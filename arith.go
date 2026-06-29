// Copyright (c) the go-ruby-bigdecimal/bigdecimal authors
//
// SPDX-License-Identifier: BSD-3-Clause

package bigdecimal

import "math/big"

// alignedCoefs returns the two significands scaled to a common exponent (the
// lower of the two), so they can be added/subtracted as plain integers.
func alignedCoefs(a, b *Decimal) (ca, cb *big.Int, exp int) {
	exp = a.exp
	if b.exp < exp {
		exp = b.exp
	}
	ca = scaleCoef(a.coef, a.exp-exp)
	cb = scaleCoef(b.coef, b.exp-exp)
	return ca, cb, exp
}

// scaleCoef multiplies coef by 10**shift (shift >= 0).
func scaleCoef(coef *big.Int, shift int) *big.Int {
	if shift == 0 {
		return new(big.Int).Set(coef)
	}
	return new(big.Int).Mul(coef, pow10(shift))
}

// pow10 returns 10**n (n >= 0).
func pow10(n int) *big.Int {
	return new(big.Int).Exp(bigTen, big.NewInt(int64(n)), nil)
}

// Add returns d + o (BigDecimal#+). Special-value combinations follow MRI:
// NaN propagates, ∞+∞ stays ∞, ∞+(−∞) is NaN.
func (d *Decimal) Add(o *Decimal) *Decimal {
	if r, ok := addSpecial(d, o); ok {
		return r
	}
	ca, cb, exp := alignedCoefs(d, o)
	sum := new(big.Int).Add(d.applySign(ca), o.applySign(cb))
	return fromSigned(sum, exp)
}

// Sub returns d - o (BigDecimal#-).
func (d *Decimal) Sub(o *Decimal) *Decimal {
	return d.Add(o.Neg())
}

// applySign returns coef negated when the receiver is negative.
func (d *Decimal) applySign(coef *big.Int) *big.Int {
	if d.sign < 0 {
		return new(big.Int).Neg(coef)
	}
	return coef
}

// fromSigned builds a finite Decimal from a signed magnitude at exponent exp.
// A zero magnitude normalises to +0 (MRI yields +0.0 for x + (−x)).
func fromSigned(v *big.Int, exp int) *Decimal {
	sign := 1
	if v.Sign() < 0 {
		sign = -1
	}
	return newFinite(sign, new(big.Int).Abs(v), exp)
}

// Neg returns -d (unary minus). A non-NaN value flips sign (including a signed
// zero, mirroring MRI's distinct +0.0 / −0.0); NaN is unchanged.
func (d *Decimal) Neg() *Decimal {
	if d.kind == nan {
		return nanDecimal()
	}
	n := d.clone()
	n.sign = -d.sign
	return n
}

// Abs returns |d| (BigDecimal#abs).
func (d *Decimal) Abs() *Decimal {
	if d.kind == nan {
		return nanDecimal()
	}
	n := d.clone()
	n.sign = 1
	return n
}

// Mul returns d * o (BigDecimal#*).
func (d *Decimal) Mul(o *Decimal) *Decimal {
	if r, ok := mulSpecial(d, o); ok {
		return r
	}
	coef := new(big.Int).Mul(d.coef, o.coef)
	return newFinite(d.sign*o.sign, coef, d.exp+o.exp)
}

// addSpecial handles the special-value cases of Add, returning ok == false when
// both operands are finite.
func addSpecial(a, b *Decimal) (*Decimal, bool) {
	if a.kind == finite && b.kind == finite {
		return nil, false
	}
	if a.kind == nan || b.kind == nan {
		return nanDecimal(), true
	}
	switch {
	case a.kind == inf && b.kind == inf:
		if a.sign == b.sign {
			return infDecimal(a.sign), true
		}
		return nanDecimal(), true // ∞ + (−∞)
	case a.kind == inf:
		return infDecimal(a.sign), true
	default: // b.kind == inf
		return infDecimal(b.sign), true
	}
}

// mulSpecial handles the special-value cases of Mul.
func mulSpecial(a, b *Decimal) (*Decimal, bool) {
	if a.kind == finite && b.kind == finite {
		return nil, false
	}
	if a.kind == nan || b.kind == nan {
		return nanDecimal(), true
	}
	// One or both are infinite. ∞ * 0 is NaN.
	if (a.kind == inf && b.kind == finite && b.coef.Sign() == 0) ||
		(b.kind == inf && a.kind == finite && a.coef.Sign() == 0) {
		return nanDecimal(), true
	}
	return infDecimal(a.sign * b.sign), true
}

// roundShift drops the low `drop` decimal digits of coef and reports whether the
// kept quotient must be incremented (magnitude rounding) under mode, given the
// value's sign. coef is the non-negative magnitude. intNonZero must report
// whether any discarded digit at an integer position (10**0 or above) is
// non-zero — it is consulted only by ROUND_UP to reproduce MRI's quirk of
// ignoring a purely-fractional discarded remainder at a whole-digit place.
func roundShift(coef *big.Int, drop int, mode RoundMode, sign int, intNonZero bool) (q *big.Int, up bool) {
	div := pow10(drop)
	q = new(big.Int)
	r := new(big.Int)
	q.QuoRem(coef, div, r)
	if r.Sign() == 0 {
		return q, false
	}
	// Compare 2*r to div to classify the full remainder as below/at/above half.
	twice := new(big.Int).Lsh(r, 1)
	cmp := twice.Cmp(div) // <0 below half, ==0 exactly half, >0 above half
	switch mode {
	case RoundDown:
		up = false
	case RoundUp:
		// MRI's ROUND_UP/DOWN ignore the fractional part of the discarded
		// remainder: at a whole-digit rounding place a sub-unit value (0.99 → tens)
		// is NOT rounded away from zero, while CEILING/FLOOR and the HALF modes do
		// see it. intNonZero is true only when a discarded *integer*-position digit
		// is non-zero.
		up = intNonZero
	case RoundCeiling:
		up = sign > 0
	case RoundFloor:
		up = sign < 0
	case RoundHalfDown:
		up = cmp > 0
	case RoundHalfEven:
		switch {
		case cmp > 0:
			up = true
		case cmp < 0:
			up = false
		default:
			up = q.Bit(0) == 1 // round to even
		}
	default: // RoundHalfUp
		up = cmp >= 0
	}
	return q, up
}
