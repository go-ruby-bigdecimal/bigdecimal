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
	d, err := parse(s)
	if err != nil {
		return nil, err
	}
	// Build the config only when options are actually supplied. Taking &cfg forces
	// it to the heap (an option func could stash the pointer), so keeping the
	// address-of inside this branch spares the plain BigDecimal("…") call — the hot
	// path — that allocation entirely.
	if len(opts) > 0 {
		var cfg config
		for _, o := range opts {
			o(&cfg)
		}
		if cfg.hasDigits && cfg.digits > 0 {
			d = d.limitDigits(cfg.digits)
		}
	}
	return d, nil
}

// parse does the literal scanning for New.
func parse(s string) (*Decimal, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		// MRI treats a blank string as zero.
		return newFiniteOwned(1, new(big.Int), 0), nil
	}
	// Specials (case-insensitive, optional sign for Infinity). matchFold compares
	// without the whole-string strings.ToLower copy the scan used to allocate.
	switch {
	case matchFold(raw, "nan"):
		return nanDecimal(), nil
	case matchFold(raw, "infinity"), matchFold(raw, "+infinity"):
		return infDecimal(1), nil
	case matchFold(raw, "-infinity"):
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
	// Only pay strings.ReplaceAll's Count sweep and Builder when a separator is
	// actually present; the overwhelmingly common literal has none.
	if strings.IndexByte(body, '_') >= 0 {
		body = strings.ReplaceAll(body, "_", "")
	}
	if body == "" {
		return nil, ErrSyntax
	}

	// Split off the exponent (e/E/d/D — MRI accepts the Fortran "d" form). The single
	// index also distinguishes the "no exponent" and "empty exponent" cases the old
	// strings.ContainsAny second sweep existed to catch.
	mant := body
	expVal := 0
	if i := indexExpChar(body); i >= 0 {
		expPart := body[i+1:]
		if expPart == "" {
			return nil, ErrSyntax // trailing exponent marker with no digits
		}
		v, err := strconv.Atoi(expPart)
		if err != nil {
			return nil, ErrSyntax
		}
		mant = body[:i]
		expVal = v
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

	// value = digits * 10**(expVal - len(fracPart)), with trailing zeros folded
	// into the exponent. buildCoef validates the two digit runs, strips leading and
	// trailing zeros and folds the significant digits into a canonical big.Int in a
	// single pass — replacing the old intPart+fracPart concatenation (a heap copy)
	// and its separate allDigits, trailing-strip, leading-strip and setDigits scans.
	coef, tz, ok := buildCoef(intPart, fracPart)
	if !ok {
		return nil, ErrSyntax
	}
	if coef.Sign() == 0 { // all-zero significand → signed zero
		return &Decimal{kind: finite, sign: sign, coef: coef, exp: 0}, nil
	}
	// The stripped significand carries no trailing power of ten, so the value is
	// already in canonical form and skips newFinite's Abs copy and normalize.
	return &Decimal{kind: finite, sign: sign, coef: coef, exp: expVal - len(fracPart) + tz}, nil
}

// buildCoef folds a mantissa's integer run intPart followed by its fraction run
// fracPart into a canonical, non-negative significand — the concatenated digits
// with leading and trailing zeros removed — in one validating pass. It returns
// the count of stripped trailing zeros (which the caller adds to the exponent,
// exactly as normalize would) and ok == false on any non-digit byte. Because the
// leading/trailing skips only ever step over '0', every other byte is visited by
// the fold, so the fold's per-digit check validates the whole input without a
// separate allDigits sweep, and no intermediate concatenation is allocated.
func buildCoef(intPart, fracPart string) (coef *big.Int, trailingZeros int, ok bool) {
	ni := len(intPart)
	total := ni + len(fracPart)
	// Trim leading zeros (valueless), walking intPart then, if wholly consumed,
	// fracPart; then trim trailing zeros (each raises the exponent), walking
	// fracPart then, if wholly consumed, intPart. Direct indexing avoids a
	// per-byte accessor closure over the two runs.
	lo := 0
	for lo < ni && intPart[lo] == '0' {
		lo++
	}
	if lo == ni {
		for lo < total && fracPart[lo-ni] == '0' {
			lo++
		}
	}
	hi := total
	for hi > ni && fracPart[hi-ni-1] == '0' {
		hi--
	}
	if hi <= ni {
		for hi > lo && intPart[hi-1] == '0' {
			hi--
		}
	}
	z := new(big.Int)
	if lo >= hi { // no significant digits → zero
		return z, 0, true
	}
	// The significant digits are intPart[iLo:iHi] followed by fracPart[fLo:fHi];
	// fold each run into z. A no-closure helper lets tmp stay a stack value.
	iLo, iHi := min(lo, ni), min(hi, ni)
	fLo, fHi := max(lo-ni, 0), max(hi-ni, 0)
	var tmp big.Int
	started, ok1 := foldRun(z, &tmp, intPart[iLo:iHi], false)
	_, ok2 := foldRun(z, &tmp, fracPart[fLo:fHi], started)
	if !ok1 || !ok2 {
		return nil, 0, false
	}
	return z, total - hi, true
}

// foldRun folds the ASCII-digit run s into the accumulator z, eighteen digits at
// a time, validating each byte. started reports whether z already holds earlier
// digits: a later chunk shifts z left by that chunk's own digit count before
// adding (so a partial chunk at the int/fraction seam still concatenates
// correctly), while the very first chunk seeds z without a shift. It returns
// whether z now holds digits and whether every byte was a decimal digit. tmp is a
// caller-owned scratch big.Int reused across chunks; it is only read by Add, so it
// need never leave the stack.
func foldRun(z, tmp *big.Int, s string, started bool) (bool, bool) {
	ok := true
	for i := 0; i < len(s); {
		end := i + setDigitsChunk
		if end > len(s) {
			end = len(s)
		}
		var v uint64
		for j := i; j < end; j++ {
			c := s[j]
			if c < '0' || c > '9' {
				ok = false
			}
			v = v*10 + uint64(c-'0')
		}
		if started {
			z.Mul(z, pow10(end-i))
		} else {
			started = true
		}
		z.Add(z, tmp.SetUint64(v))
		i = end
	}
	return started, ok
}

// setDigitsChunk is the number of decimal digits buildCoef folds into one uint64
// group; 18 nines (10**18-1) stay below 2**63, so a group accumulates without
// overflow and 10**setDigitsChunk fits the cached power table.
const setDigitsChunk = 18

// indexExpChar returns the index of the first exponent marker (e/E/d/D — MRI
// accepts the Fortran "d" form) in s, or -1. It probes with the SIMD-accelerated
// strings.IndexByte for each marker rather than the asciiSet strings.IndexAny
// builds for a rune set or a hand-rolled byte loop: 'e' is by far the common form,
// and a literal carrying two different markers is malformed anyway, so returning
// the earliest hit is all that is needed. IndexByte clears the whole no-exponent
// body — the hot case — in a few vector instructions.
func indexExpChar(s string) int {
	i := strings.IndexByte(s, 'e')
	if i < 0 {
		i = len(s)
	}
	if j := strings.IndexByte(s, 'E'); j >= 0 && j < i {
		i = j
	}
	if j := strings.IndexByte(s, 'd'); j >= 0 && j < i {
		i = j
	}
	if j := strings.IndexByte(s, 'D'); j >= 0 && j < i {
		i = j
	}
	if i == len(s) {
		return -1
	}
	return i
}

// matchFold reports whether s equals the ASCII-lowercase literal lower under
// case folding, without allocating the lowercase copy strings.ToLower would.
func matchFold(s, lower string) bool {
	if len(s) != len(lower) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != lower[i] {
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
	return newFiniteOwned(sign, new(big.Int).Abs(i), 0)
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
		q.Add(q, bigOne)
	}
	return newFiniteOwned(d.sign, q, d.exp+drop)
}
