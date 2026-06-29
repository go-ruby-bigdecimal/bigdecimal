// Copyright (c) the go-ruby-bigdecimal/bigdecimal authors
//
// SPDX-License-Identifier: BSD-3-Clause

package bigdecimal

import (
	"math/big"
	"strconv"
	"strings"
)

// New parses a BigDecimal literal — the MRI BigDecimal("…") constructor for a
// String. It accepts an optional leading sign, a decimal mantissa with an
// optional fraction and an optional "e"/"E"/"d"/"D" exponent, plus the specials
// "NaN", "Infinity"/"+Infinity"/"-Infinity" (and MRI's tolerated leading/trailing
// blanks and an underscore digit separator). With WithDigits(n) the result is
// rounded to n significant digits. An unparseable string returns ErrSyntax.
func New(s string, opts ...Option) (*Decimal, error) {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	d, err := parse(s)
	if err != nil {
		return nil, err
	}
	if cfg.hasDigits && cfg.digits > 0 {
		d = d.limitDigits(cfg.digits)
	}
	return d, nil
}

// parse does the literal scanning for New.
func parse(s string) (*Decimal, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		// MRI treats a blank string as zero.
		return newFinite(1, big.NewInt(0), 0), nil
	}
	// Specials (case-insensitive, optional sign for Infinity).
	switch strings.ToLower(raw) {
	case "nan":
		return nanDecimal(), nil
	case "infinity", "+infinity":
		return infDecimal(1), nil
	case "-infinity":
		return infDecimal(-1), nil
	}

	sign := 1
	body := raw
	switch body[0] {
	case '+':
		body = body[1:]
	case '-':
		sign = -1
		body = body[1:]
	}
	body = strings.ReplaceAll(body, "_", "")
	if body == "" {
		return nil, ErrSyntax
	}

	// Split off the exponent (e/E/d/D — MRI accepts the Fortran "d" form).
	mant := body
	expPart := ""
	if i := strings.IndexAny(body, "eEdD"); i >= 0 {
		mant = body[:i]
		expPart = body[i+1:]
	}
	expVal := 0
	if expPart != "" {
		v, err := strconv.Atoi(expPart)
		if err != nil {
			return nil, ErrSyntax
		}
		expVal = v
	} else if strings.ContainsAny(body, "eEdD") {
		return nil, ErrSyntax
	}

	// Split the mantissa into integer and fraction digit runs.
	intPart, fracPart := mant, ""
	if i := strings.IndexByte(mant, '.'); i >= 0 {
		intPart = mant[:i]
		fracPart = mant[i+1:]
	}
	if intPart == "" && fracPart == "" {
		return nil, ErrSyntax
	}
	if !allDigits(intPart) || !allDigits(fracPart) {
		return nil, ErrSyntax
	}

	// digits is non-empty (both parts empty is rejected above) and all ASCII
	// digits, so SetString always succeeds.
	coef, _ := new(big.Int).SetString(intPart+fracPart, 10)
	// value = coef * 10**(expVal - len(fracPart))
	return newFinite(sign, coef, expVal-len(fracPart)), nil
}

// allDigits reports whether s is empty or all ASCII digits.
func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// FromInt builds a Decimal from an int64 (the exact BigDecimal(Integer) case).
func FromInt(i int64) *Decimal {
	sign := 1
	if i < 0 {
		sign = -1
	}
	return newFinite(sign, new(big.Int).SetInt64(i), 0)
}

// FromBigInt builds a Decimal from an arbitrary-precision integer.
func FromBigInt(i *big.Int) *Decimal {
	sign := 1
	if i.Sign() < 0 {
		sign = -1
	}
	return newFinite(sign, new(big.Int).Abs(i), 0)
}

// FromFloat builds a Decimal from a float64 limited to n significant digits, the
// MRI BigDecimal(Float, n) case (n must be > 0; MRI requires the precision for a
// Float argument). It renders the float with n significant digits and reparses,
// so 0.1 with n digits is the n-digit decimal nearest the binary float.
func FromFloat(f float64, n int) (*Decimal, error) {
	if n <= 0 {
		return nil, ErrSyntax
	}
	s := strconv.FormatFloat(f, 'e', n-1, 64)
	return New(s)
}

// limitDigits rounds a finite value to n significant digits (half-up, MRI's
// BigDecimal(value, n) rounding). Specials and values already within n digits
// pass through unchanged.
func (d *Decimal) limitDigits(n int) *Decimal {
	if d.kind != finite || d.coef.Sign() == 0 {
		return d
	}
	cur := d.numDigits()
	if cur <= n {
		return d
	}
	drop := cur - n
	// Round coef from cur digits down to n, half-up, raising exp by drop.
	q, up := roundShift(d.coef, drop, RoundHalfUp, d.sign, false)
	if up {
		q.Add(q, big.NewInt(1))
	}
	return newFinite(d.sign, q, d.exp+drop)
}
