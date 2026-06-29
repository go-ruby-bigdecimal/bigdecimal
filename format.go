// Copyright (c) the go-ruby-bigdecimal/bigdecimal authors
//
// SPDX-License-Identifier: BSD-3-Clause

package bigdecimal

import (
	"strconv"
	"strings"
)

// String renders the value in MRI's default BigDecimal#to_s form: the canonical
// scientific notation 0.<digits>e<exp> (so BigDecimal("1.5").to_s is "0.15e1").
func (d *Decimal) String() string { return d.ToS("") }

// ToS renders the value under MRI's BigDecimal#to_s(fmt) grammar. The format
// string carries, in order:
//
//   - an optional leading sign flag: '+' prefixes non-negative values with '+',
//     a space prefixes them with ' ' (a negative value always shows '-');
//   - an optional run of digits N, which groups the significant digits in
//     blocks of N (separated by a space) — the integer part from the right and
//     the fraction from the left;
//   - an optional 'F'/'f' selecting the plain floating form (e.g. "1234.5"), or
//     'E'/'e' / nothing selecting the scientific form 0.<digits>e<exp>.
//
// The classic forms are ToS("") / ToS("E") (scientific), ToS("F") (plain),
// ToS("+") / ToS(" ") (sign prefix) and ToS("5F") (5-digit grouping).
func (d *Decimal) ToS(format string) string {
	prefix, group, plain := parseFormat(format)

	switch d.kind {
	case nan:
		return "NaN"
	case inf:
		if d.sign < 0 {
			return "-Infinity"
		}
		return prefix + "Infinity"
	}

	sign := ""
	switch {
	case d.sign < 0:
		sign = "-"
	default:
		sign = prefix
	}

	if plain {
		return sign + d.plainForm(group)
	}
	return sign + d.sciForm(group)
}

// parseFormat decodes a to_s format string into its three components. It reads a
// leading '+' or space as the sign prefix, then a run of digits as the grouping
// count, then an 'F'/'f' (plain) or 'E'/'e'/end (scientific) selector. Anything
// after the selector is ignored, matching MRI.
func parseFormat(format string) (prefix string, group int, plain bool) {
	i := 0
	if i < len(format) {
		switch format[i] {
		case '+':
			prefix = "+"
			i++
		case ' ':
			prefix = " "
			i++
		}
	}
	start := i
	for i < len(format) && format[i] >= '0' && format[i] <= '9' {
		i++
	}
	if i > start {
		group, _ = strconv.Atoi(format[start:i])
	}
	if i < len(format) && (format[i] == 'F' || format[i] == 'f') {
		plain = true
	}
	return prefix, group, plain
}

// sciForm renders the magnitude in canonical scientific notation
// 0.<digits>e<exp>, optionally grouping the digit string in blocks of `group`.
func (d *Decimal) sciForm(group int) string {
	if d.coef.Sign() == 0 {
		return "0.0"
	}
	digits := d.coef.Text(10)
	exp := d.pointExp()
	if group > 0 {
		digits = groupLeft(digits, group)
	}
	return "0." + digits + "e" + strconv.Itoa(exp)
}

// plainForm renders the magnitude in plain decimal notation (no exponent),
// optionally grouping the integer part from the right and the fraction from the
// left in blocks of `group`.
func (d *Decimal) plainForm(group int) string {
	if d.coef.Sign() == 0 {
		return "0.0"
	}
	digits := d.coef.Text(10)
	point := d.pointExp() // number of digits left of the decimal point

	var intPart, fracPart string
	switch {
	case point <= 0:
		// 0.00…digits — leading zeros before the significant digits.
		intPart = "0"
		fracPart = strings.Repeat("0", -point) + digits
	case point >= len(digits):
		// All digits are integral; pad with trailing zeros, no fraction.
		intPart = digits + strings.Repeat("0", point-len(digits))
		fracPart = ""
	default:
		intPart = digits[:point]
		fracPart = digits[point:]
	}

	if group > 0 {
		intPart = groupRight(intPart, group)
		if fracPart != "" {
			fracPart = groupLeft(fracPart, group)
		}
	}
	if fracPart == "" {
		return intPart + ".0"
	}
	return intPart + "." + fracPart
}

// groupLeft inserts a space every n characters scanning from the left.
func groupLeft(s string, n int) string {
	var b strings.Builder
	for i, c := range s {
		if i > 0 && i%n == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// groupRight inserts a space every n characters scanning from the right, so the
// left-most block may be shorter than n.
func groupRight(s string, n int) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%n == 0 {
			b.WriteByte(' ')
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
