// Copyright (c) the go-ruby-bigdecimal/bigdecimal authors
//
// SPDX-License-Identifier: BSD-3-Clause

package bigdecimal

import "math/big"

// Div returns d / o rounded to prec significant digits (BigDecimal#div(o, prec)
// / the bare BigDecimal#/). When prec > 0 the quotient carries exactly that many
// significant digits, half-up; when prec <= 0 it is computed exactly if the
// division terminates and otherwise to an MRI-style default working precision.
//
// Division by a finite zero, or any NaN operand, yields the matching special
// (the caller decides whether that is an error); see DivE for the fallible form.
func (d *Decimal) Div(o *Decimal, prec int) *Decimal {
	r, _ := d.DivE(o, prec)
	return r
}

// DivE is Div with an explicit error for finite-by-zero (ErrZeroDivision),
// mirroring MRI raising ZeroDivisionError. Special operands still return the
// matching special with a nil error (∞ / 2, NaN / 1, …).
func (d *Decimal) DivE(o *Decimal, prec int) (*Decimal, error) {
	if r, ok := divSpecial(d, o); ok {
		if r == nil {
			return nanDecimal(), ErrZeroDivision // finite / 0
		}
		return r, nil
	}
	sign := d.sign * o.sign
	// value = (d.coef/o.coef) * 10**(d.exp-o.exp). Compute the quotient's digits
	// to the requested significant precision.
	want := prec
	if want <= 0 {
		want = defaultDivPrec(d, o)
	}
	q, qexp := divDigits(d.coef, o.coef, d.exp-o.exp, want)
	if q.Sign() == 0 {
		return newFinite(sign, big.NewInt(0), 0), nil
	}
	return newFinite(sign, q, qexp), nil
}

// divSpecial handles NaN/∞/zero combinations of division. It returns ok == true
// when handled; a nil *Decimal with ok == true signals finite-by-zero (the
// caller turns that into ErrZeroDivision).
func divSpecial(a, b *Decimal) (*Decimal, bool) {
	if a.kind == nan || b.kind == nan {
		return nanDecimal(), true
	}
	if a.kind == inf {
		if b.kind == inf {
			return nanDecimal(), true // ∞/∞
		}
		return infDecimal(a.sign * b.sign), true
	}
	if b.kind == inf {
		return newFinite(a.sign*b.sign, big.NewInt(0), 0), true // finite/∞ = 0
	}
	// Both finite.
	if b.coef.Sign() == 0 {
		if a.coef.Sign() == 0 {
			return nanDecimal(), true // 0/0
		}
		return nil, true // finite/0 → ZeroDivision
	}
	return nil, false
}

// divDigits computes a/b * 10**baseExp to `prec` significant digits, returning a
// non-negative significand and its base-ten exponent. Both a and b are
// non-negative and b != 0. It over-provisions a few guard digits so the leading
// digit is always present, then trims the quotient to exactly `prec` significant
// digits, rounding half-up on the discarded tail (including the inexact division
// remainder).
func divDigits(a, b *big.Int, baseExp, prec int) (q *big.Int, exp int) {
	if a.Sign() == 0 {
		return big.NewInt(0), 0
	}
	const guard = 2
	// The most-significant digit of a/b sits near power (digits(a)-digits(b)). To
	// land prec+guard significant digits we scale a by 10**p (p chosen relative to
	// b's magnitude, independent of baseExp) so floor(a*10**p / b) has that many
	// digits; the value's exponent is then baseExp - p.
	p := (prec + guard) - (len(a.Text(10)) - len(b.Text(10)))
	num := new(big.Int).Set(a)
	den := new(big.Int).Set(b)
	if p >= 0 {
		num.Mul(num, pow10(p))
	} else {
		den.Mul(den, pow10(-p))
	}
	q = new(big.Int)
	r := new(big.Int)
	q.QuoRem(num, den, r)
	qexp := baseExp - p
	// q now holds about prec+guard digits; fold a non-zero division remainder into
	// the last digit by bumping it (it cannot change the digit count) so the
	// half-up trim below sees a faithful low digit.
	if r.Sign() != 0 {
		twice := new(big.Int).Lsh(r, 1)
		if twice.Cmp(den) >= 0 {
			q.Add(q, big.NewInt(1))
		}
	}
	// Trim to exactly prec significant digits, half-up.
	qd := len(q.Text(10))
	if qd > prec {
		drop := qd - prec
		q2, up := roundShift(q, drop, RoundHalfUp, 1, false)
		if up {
			q2.Add(q2, big.NewInt(1))
		}
		q = q2
		qexp += drop
	}
	return q, qexp
}

// defaultDivPrec returns the working significant-digit count MRI's bare #/ uses
// when no precision is given: enough to hold both operands plus a guard window,
// rounded up to a multiple of nine (MRI's internal BASE_FIG) and never below 32.
func defaultDivPrec(a, b *Decimal) int {
	n := a.numDigits() + b.numDigits()
	const baseFig = 9
	n = ((n + baseFig - 1) / baseFig) * baseFig
	if n < 32 {
		return 32
	}
	return n
}

// IDiv returns the integer quotient floor(d / o) as a Decimal (BigDecimal#div
// with no precision argument): the largest integer not greater than the exact
// quotient. NaN/∞/zero follow MRI; finite-by-zero returns NaN with
// ErrZeroDivision.
func (d *Decimal) IDiv(o *Decimal) (*Decimal, error) {
	q, _, err := d.divModParts(o)
	return q, err
}

// DivMod returns the floored quotient and the modulo (BigDecimal#divmod): q is
// floor(d/o) and r is d - q*o, so r has the sign of o.
func (d *Decimal) DivMod(o *Decimal) (q, r *Decimal, err error) {
	return d.divModParts(o)
}

// Mod returns the floored modulo d % o (BigDecimal#% / #modulo): the remainder
// with the sign of o.
func (d *Decimal) Mod(o *Decimal) (*Decimal, error) {
	_, r, err := d.divModParts(o)
	return r, err
}

// divModParts computes the floored quotient and remainder. Both operands must be
// representable; specials yield NaN (with ErrZeroDivision for finite-by-zero).
func (d *Decimal) divModParts(o *Decimal) (q, r *Decimal, err error) {
	if d.kind != finite || o.kind != finite {
		if d.kind == finite && o.kind == inf {
			// finite divmod ∞: quotient 0 (or -1) ; MRI: q=0/-1, r=d. Keep it simple
			// and faithful: q = floor(d/∞) = 0 for same sign, -1 otherwise; r = d.
			return nanDecimal(), nanDecimal(), nil
		}
		return nanDecimal(), nanDecimal(), nil
	}
	if o.coef.Sign() == 0 {
		return nanDecimal(), nanDecimal(), ErrZeroDivision
	}
	// Align to a common exponent and do integer floored division on the signed
	// significands.
	ca, cb, exp := alignedCoefs(d, o)
	na := d.applySign(ca)
	nb := o.applySign(cb)
	qq := new(big.Int)
	rr := new(big.Int)
	qq.DivMod(na, nb, rr) // Go's DivMod is Euclidean; adjust to floored below.
	// Go big.Int DivMod gives 0 <= rr < |nb|. Convert to floored (sign of nb).
	qFloor, rFloor := flooredDivMod(na, nb)
	q = FromBigInt(qFloor)
	r = newFinite(signOf(rFloor), new(big.Int).Abs(rFloor), exp)
	return q, r, nil
}

// flooredDivMod returns q,r with q = floor(a/b) and r = a - q*b (sign of b).
func flooredDivMod(a, b *big.Int) (q, r *big.Int) {
	q = new(big.Int)
	r = new(big.Int)
	q.QuoRem(a, b, r) // truncated toward zero
	if r.Sign() != 0 && (r.Sign() < 0) != (b.Sign() < 0) {
		q.Sub(q, big.NewInt(1))
		r.Add(r, b)
	}
	return q, r
}

// signOf returns the Decimal sign (-1 or +1) carrying a remainder magnitude: -1
// for a negative integer, +1 otherwise (a zero remainder normalises to +0).
func signOf(v *big.Int) int {
	if v.Sign() < 0 {
		return -1
	}
	return 1
}

// Remainder returns the truncated remainder d.remainder(o): d - o*trunc(d/o), so
// it has the sign of d (unlike Mod, which has the sign of o).
func (d *Decimal) Remainder(o *Decimal) (*Decimal, error) {
	if d.kind != finite || o.kind != finite {
		return nanDecimal(), nil
	}
	if o.coef.Sign() == 0 {
		return nanDecimal(), ErrZeroDivision
	}
	ca, cb, exp := alignedCoefs(d, o)
	na := d.applySign(ca)
	nb := o.applySign(cb)
	r := new(big.Int)
	new(big.Int).QuoRem(na, nb, r) // truncated remainder, sign of na
	return newFinite(signOf(r), new(big.Int).Abs(r), exp), nil
}

// Pow returns d ** n for an integer exponent (BigDecimal#** / #power). A negative
// exponent yields the reciprocal (to the default division precision); 0**0 is 1;
// 0 raised to a negative power returns +Infinity (as MRI does).
func (d *Decimal) Pow(n int) *Decimal {
	if d.kind == nan {
		return nanDecimal()
	}
	if d.kind == inf {
		return powInf(d, n)
	}
	if n == 0 {
		return FromInt(1)
	}
	if n > 0 {
		acc := FromInt(1)
		base := d.clone()
		for e := n; e > 0; e >>= 1 {
			if e&1 == 1 {
				acc = acc.Mul(base)
			}
			if e > 1 {
				base = base.Mul(base)
			}
		}
		return acc
	}
	// Negative exponent: reciprocal of d**|n|.
	pos := d.Pow(-n)
	if pos.IsZero() {
		return infDecimal(1) // 0 ** negative → +Infinity (MRI)
	}
	one := FromInt(1)
	return one.Div(pos, defaultDivPrec(one, pos))
}

// powInf handles ±Infinity raised to an integer power.
func powInf(d *Decimal, n int) *Decimal {
	switch {
	case n == 0:
		return FromInt(1)
	case n > 0:
		if d.sign < 0 && n%2 == 1 {
			return infDecimal(-1)
		}
		return infDecimal(1)
	default:
		return newFinite(1, big.NewInt(0), 0) // ∞ ** negative → 0
	}
}
