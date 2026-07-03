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
	// bigOne is a shared, read-only 1 used as an increment/decrement operand
	// (big.Int.Add/Sub never mutate their inputs), sparing a big.NewInt(1)
	// allocation on the rounding and division paths.
	bigOne = big.NewInt(1)
)

// pow10Cache holds 10**0 … 10**pow10CacheMax, built once at init. pow10 returns
// the cached, SHARED value for an in-range exponent, so callers MUST treat the
// result as immutable (read-only): scale/round/divide only ever pass it as a
// multiplicand, divisor or comparand, never mutate it. Powers of ten dominate
// every scaling, rounding and division step, so caching them removes the bulk of
// the per-op math/big.Int.Exp allocation the hot paths used to pay.
const pow10CacheMax = 512

var pow10Cache [pow10CacheMax + 1]*big.Int

func init() {
	pow10Cache[0] = big.NewInt(1)
	for i := 1; i <= pow10CacheMax; i++ {
		pow10Cache[i] = new(big.Int).Mul(pow10Cache[i-1], bigTen)
	}
}

// pow10 returns 10**n (n >= 0). In-range exponents return a shared, immutable
// cached value; larger ones are computed fresh.
func pow10(n int) *big.Int {
	if n >= 0 && n <= pow10CacheMax {
		return pow10Cache[n]
	}
	return new(big.Int).Exp(bigTen, big.NewInt(int64(n)), nil)
}

// decDigits returns the number of decimal digits in the non-negative integer x
// (a zero counts as one digit), without allocating the base-ten string that
// big.Int.Text would. It seeds an upper bound from the bit length — a b-bit
// integer has at most ⌊b·log10(2)⌋+1 digits — then walks it down against the
// cached powers of ten, so the result is exact for every magnitude while the
// common case costs one BitLen and a single (allocation-free) comparison.
func decDigits(x *big.Int) int {
	if x.Sign() == 0 {
		return 1
	}
	// 1234/4096 = 0.30127 is strictly greater than log10(2) = 0.30103, so
	// ⌊bits·1234/4096⌋+1 never under-counts the digits at any magnitude (a lower
	// coefficient would under-count huge values); the loop then trims that upper
	// bound — at most a step or two — down to the exact count.
	n := x.BitLen()*1234>>12 + 1
	for n > 1 && x.Cmp(pow10(n-1)) < 0 {
		n--
	}
	return n
}

// nanDecimal / infDecimal build the specials.
func nanDecimal() *Decimal { return &Decimal{kind: nan, sign: 1, coef: new(big.Int)} }

func infDecimal(sign int) *Decimal { return &Decimal{kind: inf, sign: sign, coef: new(big.Int)} }

// newFinite builds a finite Decimal from a magnitude and exponent, then
// normalises it (strips trailing zero digits, collapses a zero significand). It
// copies coef, so the caller keeps ownership; a hot-path caller that has a fresh,
// non-negative magnitude it can surrender should use newFiniteOwned instead.
func newFinite(sign int, coef *big.Int, exp int) *Decimal {
	return newFiniteOwned(sign, new(big.Int).Abs(coef), exp)
}

// newFiniteOwned is newFinite for a caller that hands over a fresh, non-negative
// magnitude the constructor may keep and reuse. It avoids newFinite's defensive
// Abs copy — the single biggest per-op allocation on the add/div/parse paths,
// each of which already produces exactly such a value.
func newFiniteOwned(sign int, coef *big.Int, exp int) *Decimal {
	d := &Decimal{kind: finite, sign: sign, coef: coef, exp: exp}
	d.normalize()
	return d
}

// normalize removes a trailing power of ten from coef (rolling it into exp) so
// the canonical form is unique, and zeroes the exponent of a zero significand. It
// is only ever called on a freshly-built finite value.
//
// An odd significand can carry no factor of ten, so the parity check retires the
// common case with no allocation at all; when zeros must be stripped the two
// scratch integers are swapped between coef and the quotient so the loop
// allocates a fixed two words regardless of how many zeros fall away. It never
// writes through the incoming coef pointer, so a shared cache value would be safe
// even though callers always pass an owned one.
func (d *Decimal) normalize() {
	if d.coef.Sign() == 0 {
		d.exp = 0
		return
	}
	if d.coef.Bit(0) != 0 {
		return // odd ⇒ not divisible by ten
	}
	q := new(big.Int)
	r := new(big.Int)
	for {
		q.QuoRem(d.coef, bigTen, r)
		if r.Sign() != 0 {
			break
		}
		d.coef, q = q, d.coef
		d.exp++
	}
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
	return decDigits(d.coef)
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
