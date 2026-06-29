// Copyright (c) the go-ruby-bigdecimal/bigdecimal authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package bigdecimal is a pure-Go (no cgo) reimplementation of Ruby's
// BigDecimal: arbitrary-precision decimal arithmetic and MRI-byte-exact
// formatting. It is the decimal backend for go-embedded-ruby, but a standalone,
// reusable module — a sibling of go-ruby-regexp, go-ruby-erb and go-ruby-yaml.
//
// A Decimal is sign + significand (an arbitrary-precision integer) + a base-ten
// exponent, plus the three IEEE-style specials NaN / +Infinity / -Infinity that
// MRI's BigDecimal carries. The underlying integer math uses math/big; the
// decimal semantics — the canonical 0.15e1 scientific to_s, the rounding modes,
// floor/ceil/truncate at an arbitrary digit, div with an explicit precision,
// divmod/modulo/remainder, and the to_s("F"/"E"/"+"/" "/"N") format grammar —
// are this package's own, ported from MRI 4.0.5.
package bigdecimal

import (
	"errors"
	"math/big"
)

// kind distinguishes a finite value from the three BigDecimal specials.
type kind uint8

const (
	finite kind = iota
	nan
	inf // sign field carries the direction (+1 / -1)
)

// Decimal is an arbitrary-precision decimal: value = sign * coef * 10**exp for a
// finite number, where coef is a non-negative significand. The zero Decimal is
// not valid; build values through New / FromInt / FromBigInt.
//
// A finite value is kept normalised so that coef has no trailing factor of ten
// (coef == 0 ⇒ exp == 0), which makes the canonical scientific to_s and equality
// independent of how the value was written. sign is +1 or -1 (a zero keeps the
// sign it was parsed with, mirroring MRI's distinct +0.0 / -0.0).
type Decimal struct {
	kind kind
	sign int // +1 or -1
	coef *big.Int
	exp  int // value = sign * coef * 10**exp
}

// RoundMode selects how Round (and the precision-limited Div) breaks a tie or
// drops digits — the BigDecimal::ROUND_* family.
type RoundMode int

const (
	// RoundUp rounds away from zero (ROUND_UP).
	RoundUp RoundMode = iota
	// RoundDown truncates toward zero (ROUND_DOWN).
	RoundDown
	// RoundHalfUp rounds a tie away from zero (ROUND_HALF_UP) — the default.
	RoundHalfUp
	// RoundHalfEven rounds a tie to the even neighbour (ROUND_HALF_EVEN, banker's).
	RoundHalfEven
	// RoundHalfDown rounds a tie toward zero (ROUND_HALF_DOWN).
	RoundHalfDown
	// RoundCeiling rounds toward +Infinity (ROUND_CEILING).
	RoundCeiling
	// RoundFloor rounds toward -Infinity (ROUND_FLOOR).
	RoundFloor
)

// Option configures New; the variadic mirrors BigDecimal(value, ndigits)'s
// optional significant-digit limit.
type Option func(*config)

type config struct {
	hasDigits bool
	digits    int
}

// WithDigits limits a constructed value to n significant digits, as MRI's
// BigDecimal(value, n) does. n == 0 means "as many digits as the input has".
func WithDigits(n int) Option {
	return func(c *config) { c.hasDigits = true; c.digits = n }
}

var (
	// ErrSyntax is returned by New for a string that is not a BigDecimal literal.
	ErrSyntax = errors.New("invalid BigDecimal value")
	// ErrZeroDivision is returned by the fallible division/modulo helpers for a
	// finite-by-zero operation (MRI raises ZeroDivisionError).
	ErrZeroDivision = errors.New("divided by 0")
)

var (
	bigTen  = big.NewInt(10)
	bigZero = big.NewInt(0)
)

// nanDecimal / infDecimal build the specials.
func nanDecimal() *Decimal { return &Decimal{kind: nan, sign: 1, coef: new(big.Int)} }

func infDecimal(sign int) *Decimal { return &Decimal{kind: inf, sign: sign, coef: new(big.Int)} }

// newFinite builds a finite Decimal from a magnitude and exponent, then
// normalises it (strips trailing zero digits, collapses a zero significand).
func newFinite(sign int, coef *big.Int, exp int) *Decimal {
	d := &Decimal{kind: finite, sign: sign, coef: new(big.Int).Abs(coef), exp: exp}
	d.normalize()
	return d
}

// normalize removes a trailing power of ten from coef (rolling it into exp) so
// the canonical form is unique, and zeroes the exponent of a zero significand. It
// is only ever called on a freshly-built finite value (from newFinite).
func (d *Decimal) normalize() {
	if d.coef.Sign() == 0 {
		d.exp = 0
		return
	}
	m := new(big.Int).Abs(d.coef)
	q := new(big.Int)
	r := new(big.Int)
	for {
		q.QuoRem(m, bigTen, r)
		if r.Sign() != 0 {
			break
		}
		m.Set(q)
		d.exp++
	}
	d.coef = m
}

// clone returns an independent copy.
func (d *Decimal) clone() *Decimal {
	return &Decimal{kind: d.kind, sign: d.sign, coef: new(big.Int).Set(d.coef), exp: d.exp}
}

// IsNaN reports whether the value is NaN.
func (d *Decimal) IsNaN() bool { return d.kind == nan }

// IsInfinite reports whether the value is ±Infinity; +1 for +Inf, -1 for -Inf,
// 0 for a finite value or NaN (matching MRI's Float#infinite? / BigDecimal#infinite?).
func (d *Decimal) IsInfinite() int {
	if d.kind == inf {
		return d.sign
	}
	return 0
}

// IsFinite reports whether the value is neither NaN nor ±Infinity.
func (d *Decimal) IsFinite() bool { return d.kind == finite }

// IsZero reports whether the value is a finite zero.
func (d *Decimal) IsZero() bool { return d.kind == finite && d.coef.Sign() == 0 }

// numDigits returns the number of decimal digits in coef. Callers guard against
// a zero significand (its "length" is meaningless), so this is only reached for a
// non-zero magnitude.
func (d *Decimal) numDigits() int {
	return len(d.coef.Text(10))
}

// pointExp is MRI's #exponent: value == 0.<digits> * 10**pointExp, i.e. the
// power of ten just above the most-significant digit. It is numDigits + exp for
// a non-zero value and 0 for a zero.
func (d *Decimal) pointExp() int {
	if d.coef.Sign() == 0 {
		return 0
	}
	return d.numDigits() + d.exp
}
