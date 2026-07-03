// Copyright (c) the go-ruby-bigdecimal/bigdecimal authors
//
// SPDX-License-Identifier: BSD-3-Clause

package bigdecimal

import (
	"math/big"
	"testing"
)

// TestDecDigitsAndPow10 white-box tests the two allocation-avoiding helpers on the
// hot paths: decDigits must equal big.Int.Text's length for every magnitude —
// including values just at and just below a power of ten, which exercise both
// bit-length-estimate correction loops — and pow10 must equal 10**n both inside
// its cache and past the cache bound.
func TestDecDigitsAndPow10(t *testing.T) {
	// A dense sweep across several power-of-ten boundaries forces both estimate
	// corrections (the seed can land one digit high or low near a power of ten).
	for i := int64(0); i <= 5000; i++ {
		x := big.NewInt(i)
		if got, want := decDigits(x), len(x.Text(10)); got != want {
			t.Errorf("decDigits(%d) = %d, want %d", i, got, want)
		}
	}
	for _, s := range []string{
		"999999999999999999", "1000000000000000000",
		"9999999999999999999999999999", "10000000000000000000000000000",
	} {
		x, _ := new(big.Int).SetString(s, 10)
		if got, want := decDigits(x), len(x.Text(10)); got != want {
			t.Errorf("decDigits(%s) = %d, want %d", s, got, want)
		}
	}
	for _, n := range []int{0, 1, 18, pow10CacheMax, pow10CacheMax + 1, 700} {
		want := new(big.Int).Exp(bigTen, big.NewInt(int64(n)), nil)
		if pow10(n).Cmp(want) != 0 {
			t.Errorf("pow10(%d) mismatch", n)
		}
	}
}

// TestLargeExponentAdd drives pow10 past its cache bound through the public Add
// path: aligning operands 600 decades apart scales one significand by 10**600.
func TestLargeExponentAdd(t *testing.T) {
	got := mustNew(t, "1e600").Add(mustNew(t, "1")).ToS("F")
	want := "1" + repeat0(599) + "1.0" // 10**600 + 1
	if got != want {
		t.Errorf("1e600 + 1 = %s… (len %d), want len %d", got[:20], len(got), len(want))
	}
}

func repeat0(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '0'
	}
	return string(b)
}

// TestNegativeSubResult drives the negative-magnitude branch of fromSignedOwned.
func TestNegativeSubResult(t *testing.T) {
	if got := mustNew(t, "0.1").Sub(mustNew(t, "0.2")).String(); got != "-0.1e0" {
		t.Errorf("0.1-0.2 = %s", got)
	}
	if got := mustNew(t, "1").Sub(mustNew(t, "5")).String(); got != "-0.4e1" {
		t.Errorf("1-5 = %s", got)
	}
}

// TestIntRatNegativeFraction covers the negative-and-fractional conversion paths.
func TestIntRatNegativeFraction(t *testing.T) {
	if mustNew(t, "-12.9").Int().Int64() != -12 {
		t.Error("to_i -12.9")
	}
	if mustNew(t, "0.25").Int().Int64() != 0 {
		t.Error("to_i 0.25")
	}
	if mustNew(t, "-3.14").Rat().RatString() != "-157/50" {
		t.Errorf("to_r -3.14 = %s", mustNew(t, "-3.14").Rat().RatString())
	}
}

// TestDivLargeOverSmall drives the p<0 branch (a dividend with many significant
// digits divided at a small precision, so the numerator scale goes negative).
func TestDivLargeOverSmall(t *testing.T) {
	if got := mustNew(t, "1234567").Div(mustNew(t, "3"), 2).String(); got != "0.41e6" {
		t.Errorf("1234567/3@2 = %s", got)
	}
	// A division whose guard tail rounds the kept digits up.
	if got := mustNew(t, "2").Div(mustNew(t, "3"), 5).String(); got != "0.66667e0" {
		t.Errorf("2/3@5 = %s", got)
	}
}

// TestDefaultPrecLarge drives the n>=32 branch of defaultDivPrec (operands whose
// combined digit count exceeds 32).
func TestDefaultPrecLarge(t *testing.T) {
	big := "12345678901234567890" // 20 digits
	got := mustNew(t, big+"."+big).Div(mustNew(t, "7"), 0)
	// MRI default precision grows past 32 here; just assert it carries > 32 digits.
	if got.Precision() <= 32 {
		t.Errorf("default-prec division kept only %d digits", got.Precision())
	}
}

// TestRemainderPositive drives signOf's positive branch and the positive-quotient
// floored divmod path.
func TestRemainderPositive(t *testing.T) {
	q, r, _ := mustNew(t, "17").DivMod(mustNew(t, "5"))
	if q.String() != "0.3e1" || r.String() != "0.2e1" {
		t.Errorf("17 divmod 5 = %s,%s", q, r)
	}
	rem, _ := mustNew(t, "17").Remainder(mustNew(t, "5"))
	if rem.String() != "0.2e1" {
		t.Errorf("17 remainder 5 = %s", rem)
	}
	// An exact divmod yields a zero remainder (signOf of zero → +).
	q, r, _ = mustNew(t, "20").DivMod(mustNew(t, "5"))
	if q.String() != "0.4e1" || !r.IsZero() {
		t.Errorf("20 divmod 5 = %s,%s", q, r)
	}
}

// TestComparisonNegatives covers cmpFinite for two negative magnitudes and the
// negative-bucket orderKey path.
func TestComparisonNegatives(t *testing.T) {
	if mustNew(t, "-2.5").Cmp(mustNew(t, "-2.5")) != 0 {
		t.Error("-2.5 <=> -2.5")
	}
	if mustNew(t, "-1").Cmp(mustNew(t, "-2")) != 1 {
		t.Error("-1 <=> -2")
	}
	if mustNew(t, "-1").Cmp(mustNew(t, "1")) != -1 {
		t.Error("-1 <=> 1")
	}
}

// TestZeroDivModExact covers a zero numerator under DivMod (zero quotient and
// remainder).
func TestZeroDivMod(t *testing.T) {
	q, r, err := mustNew(t, "0").DivMod(mustNew(t, "5"))
	if err != nil || !q.IsZero() || !r.IsZero() {
		t.Errorf("0 divmod 5 = %s,%s,%v", q, r, err)
	}
}
