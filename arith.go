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

// signedScaled returns sign * coef * 10**(d.exp-exp) as a fresh, owned big.Int
// (exp is the common exponent, never above d.exp). It fuses the scale-to-common
// exponent and the sign application into a single allocation, where the old
// alignedCoefs+applySign pair copied the significand and then, for a negative
// value, allocated a second negated copy.
func (d *Decimal) signedScaled(exp int) *big.Int {
	v := new(big.Int)
	if d.exp == exp {
		v.Set(d.coef)
	} else {
		v.Mul(d.coef, pow10(d.exp-exp))
	}
	if d.sign < 0 {
		v.Neg(v)
	}
	return v
}

// Add returns d + o (BigDecimal#+). Special-value combinations follow MRI:
// NaN propagates, ∞+∞ stays ∞, ∞+(−∞) is NaN.
func (d *Decimal) Add(o *Decimal) *Decimal {
	if r, ok := addSpecial(d, o); ok {
		return r
	}
	exp := d.exp
	if o.exp < exp {
		exp = o.exp
	}
	sum := d.signedScaled(exp)
	sum.Add(sum, o.signedScaled(exp))
	return fromSignedOwned(sum, exp)
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

// fromSignedOwned builds a finite Decimal from a signed magnitude at exponent exp
// for a caller that surrenders v: it reads the sign and then folds the magnitude
// in place (v.Abs(v)) rather than allocating a separate non-negative copy. A zero
// magnitude normalises to +0 (MRI yields +0.0 for x + (−x)).
func fromSignedOwned(v *big.Int, exp int) *Decimal {
	sign := 1
	if v.Sign() < 0 {
		sign = -1
	}
	v.Abs(v)
	return newFiniteOwned(sign, v, exp)
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
	return newFiniteOwned(d.sign*o.sign, coef, d.exp+o.exp)
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
