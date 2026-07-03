// Copyright (c) the go-ruby-bigdecimal/bigdecimal authors
//
// SPDX-License-Identifier: BSD-3-Clause

package bigdecimal

import (
	"math/big"
	"testing"
)

// mustNew parses s or fails the test.
func mustNew(t *testing.T, s string) *Decimal {
	t.Helper()
	d, err := New(s)
	if err != nil {
		t.Fatalf("New(%q): %v", s, err)
	}
	return d
}

func TestNewAndToSDefault(t *testing.T) {
	cases := []struct{ in, def, f string }{
		{"1.5", "0.15e1", "1.5"},
		{"3", "0.3e1", "3.0"},
		{"0", "0.0", "0.0"},
		{"10", "0.1e2", "10.0"},
		{"100", "0.1e3", "100.0"},
		{"0.1", "0.1e0", "0.1"},
		{"0.001", "0.1e-2", "0.001"},
		{"12.34", "0.1234e2", "12.34"},
		{"-2.5", "-0.25e1", "-2.5"},
		{"1234567890", "0.123456789e10", "1234567890.0"},
		{"0.0001234", "0.1234e-3", "0.0001234"},
		{"-0", "-0.0", "-0.0"},
		{"+5", "0.5e1", "5.0"},
		{"1_000", "0.1e4", "1000.0"},
		{"1.5e3", "0.15e4", "1500.0"},
		{"1.5E3", "0.15e4", "1500.0"},
		{"15d-1", "0.15e1", "1.5"},
		{"1.5D3", "0.15e4", "1500.0"},
		{"  2.5  ", "0.25e1", "2.5"},
		{".5", "0.5e0", "0.5"},
		{"5.", "0.5e1", "5.0"},
	}
	for _, c := range cases {
		d := mustNew(t, c.in)
		if got := d.String(); got != c.def {
			t.Errorf("New(%q).String() = %q, want %q", c.in, got, c.def)
		}
		if got := d.ToS("F"); got != c.f {
			t.Errorf("New(%q).ToS(F) = %q, want %q", c.in, got, c.f)
		}
	}
}

func TestNewSyntaxError(t *testing.T) {
	for _, s := range []string{"abc", "1.2.3", "1e", "--1", "1e1.5", "1ee2", "+", "."} {
		if _, err := New(s); err == nil {
			t.Errorf("New(%q) expected error, got nil", s)
		}
	}
	// Blank parses as zero (MRI behaviour).
	if d, err := New("   "); err != nil || !d.IsZero() {
		t.Errorf("New(blank) = %v, %v; want zero", d, err)
	}
}

func TestSpecialsParse(t *testing.T) {
	if !mustNew(t, "NaN").IsNaN() {
		t.Error("NaN not NaN")
	}
	if mustNew(t, "Infinity").IsInfinite() != 1 {
		t.Error("Infinity")
	}
	if mustNew(t, "+Infinity").IsInfinite() != 1 {
		t.Error("+Infinity")
	}
	if mustNew(t, "-Infinity").IsInfinite() != -1 {
		t.Error("-Infinity")
	}
	if mustNew(t, "nan").String() != "NaN" {
		t.Error("nan to_s")
	}
	if mustNew(t, "Infinity").String() != "Infinity" {
		t.Error("inf to_s")
	}
	if mustNew(t, "-Infinity").String() != "-Infinity" {
		t.Error("-inf to_s")
	}
	if mustNew(t, "NaN").ToS("F") != "NaN" {
		t.Error("NaN F")
	}
	if mustNew(t, "Infinity").ToS("F") != "Infinity" {
		t.Error("Inf F")
	}
}

func TestFromIntBigIntFloat(t *testing.T) {
	if FromInt(42).String() != "0.42e2" {
		t.Errorf("FromInt(42) = %s", FromInt(42).String())
	}
	if FromInt(-7).String() != "-0.7e1" {
		t.Errorf("FromInt(-7) = %s", FromInt(-7).String())
	}
	bi, _ := new(big.Int).SetString("123456789012345678901234567890", 10)
	if got := FromBigInt(bi).ToS("F"); got != "123456789012345678901234567890.0" {
		t.Errorf("FromBigInt = %s", got)
	}
	nbi, _ := new(big.Int).SetString("-99", 10)
	if FromBigInt(nbi).Sign() != -2 {
		t.Error("FromBigInt negative sign")
	}
	d, err := FromFloat(0.1, 10)
	if err != nil || d.String() != "0.1e0" {
		t.Errorf("FromFloat(0.1,10) = %v, %v", d, err)
	}
	if _, err := FromFloat(0.1, 0); err == nil {
		t.Error("FromFloat with non-positive digits should error")
	}
}

func TestWithDigits(t *testing.T) {
	d, _ := New("123456789", WithDigits(5))
	if d.String() != "0.12346e9" {
		t.Errorf("WithDigits(5) = %s", d.String())
	}
	// Fewer digits than the value: passes through.
	d2, _ := New("12.3", WithDigits(10))
	if d2.String() != "0.123e2" {
		t.Errorf("WithDigits(10) = %s", d2.String())
	}
	// Zero digits: no limiting.
	d3, _ := New("123456789", WithDigits(0))
	if d3.String() != "0.123456789e9" {
		t.Errorf("WithDigits(0) = %s", d3.String())
	}
	// Limiting a special / zero is a no-op.
	z, _ := New("0", WithDigits(3))
	if !z.IsZero() {
		t.Error("WithDigits on zero")
	}
	n, _ := New("NaN", WithDigits(3))
	if !n.IsNaN() {
		t.Error("WithDigits on NaN")
	}
}

func TestArithmetic(t *testing.T) {
	cases := []struct {
		op         string
		a, b, want string
	}{
		{"+", "0.1", "0.2", "0.3e0"},
		{"+", "1", "-1", "0.0"},
		{"-", "1", "0.0001", "0.9999e0"},
		{"*", "12.34", "100", "0.1234e4"},
		{"*", "-2", "3", "-0.6e1"},
		{"*", "0", "5", "0.0"},
	}
	for _, c := range cases {
		a, b := mustNew(t, c.a), mustNew(t, c.b)
		var got *Decimal
		switch c.op {
		case "+":
			got = a.Add(b)
		case "-":
			got = a.Sub(b)
		case "*":
			got = a.Mul(b)
		}
		if got.String() != c.want {
			t.Errorf("%s %s %s = %s, want %s", c.a, c.op, c.b, got.String(), c.want)
		}
	}
}

func TestNegAbs(t *testing.T) {
	if mustNew(t, "5").Neg().String() != "-0.5e1" {
		t.Error("Neg")
	}
	if mustNew(t, "-5").Abs().String() != "0.5e1" {
		t.Error("Abs")
	}
	if !mustNew(t, "NaN").Neg().IsNaN() {
		t.Error("Neg NaN")
	}
	if !mustNew(t, "NaN").Abs().IsNaN() {
		t.Error("Abs NaN")
	}
	if mustNew(t, "Infinity").Neg().IsInfinite() != -1 {
		t.Error("Neg Inf")
	}
	if mustNew(t, "0").Neg().String() != "-0.0" {
		t.Error("Neg zero sign")
	}
}

func TestSpecialArithmetic(t *testing.T) {
	inf := mustNew(t, "Infinity")
	ninf := mustNew(t, "-Infinity")
	nan := mustNew(t, "NaN")
	one := mustNew(t, "1")
	zero := mustNew(t, "0")

	if inf.Add(one).String() != "Infinity" {
		t.Error("inf+1")
	}
	if !inf.Sub(inf).IsNaN() {
		t.Error("inf-inf")
	}
	if !inf.Add(ninf).IsNaN() {
		t.Error("inf+(-inf)")
	}
	if one.Add(inf).String() != "Infinity" {
		t.Error("1+inf")
	}
	if !nan.Add(one).IsNaN() {
		t.Error("nan+1")
	}
	if !inf.Mul(zero).IsNaN() {
		t.Error("inf*0")
	}
	if !zero.Mul(inf).IsNaN() {
		t.Error("0*inf")
	}
	if ninf.Mul(mustNew(t, "2")).String() != "-Infinity" {
		t.Error("-inf*2")
	}
	if !nan.Mul(one).IsNaN() {
		t.Error("nan*1")
	}
	if ninf.Add(ninf).String() != "-Infinity" {
		t.Error("-inf + -inf")
	}
	if inf.Mul(inf).String() != "Infinity" {
		t.Error("inf*inf")
	}
}

func TestDiv(t *testing.T) {
	if got := mustNew(t, "1").Div(mustNew(t, "3"), 10).String(); got != "0.3333333333e0" {
		t.Errorf("1/3@10 = %s", got)
	}
	if got := mustNew(t, "2").Div(mustNew(t, "7"), 20).String(); got != "0.28571428571428571429e0" {
		t.Errorf("2/7@20 = %s", got)
	}
	if got := mustNew(t, "13").Div(mustNew(t, "4"), 0).String(); got != "0.325e1" {
		t.Errorf("13/4 = %s", got)
	}
	if got := mustNew(t, "1").Div(mustNew(t, "3"), 0).String(); got != "0.33333333333333333333333333333333e0" {
		t.Errorf("1/3 default = %s", got)
	}
	if got := mustNew(t, "100").Div(mustNew(t, "7"), 0).String(); got != "0.14285714285714285714285714285714e2" {
		t.Errorf("100/7 default = %s", got)
	}
	if !mustNew(t, "0").Div(mustNew(t, "5"), 5).IsZero() {
		t.Error("0/5")
	}
	if got := mustNew(t, "123.456").Div(mustNew(t, "78.9"), 0).String(); got != "0.1564714828897338403041825095057e1" {
		t.Errorf("123.456/78.9 default = %s", got)
	}
	if got := mustNew(t, "1").Div(mustNew(t, "30000000000"), 0).String(); got != "0.33333333333333333333333333333333e-10" {
		t.Errorf("1/30000000000 default = %s", got)
	}
	if got := mustNew(t, "5").Div(mustNew(t, "1000"), 3).String(); got != "0.5e-2" {
		t.Errorf("5/1000@3 = %s", got)
	}
}

func TestDivErrors(t *testing.T) {
	if _, err := mustNew(t, "1").DivE(mustNew(t, "0"), 5); err != ErrZeroDivision {
		t.Errorf("1/0 err = %v", err)
	}
	if d, err := mustNew(t, "0").DivE(mustNew(t, "0"), 5); err != nil || !d.IsNaN() {
		t.Errorf("0/0 = %v,%v", d, err)
	}
	if d, _ := mustNew(t, "Infinity").DivE(mustNew(t, "2"), 5); d.IsInfinite() != 1 {
		t.Error("inf/2")
	}
	if d, _ := mustNew(t, "1").DivE(mustNew(t, "Infinity"), 5); !d.IsZero() {
		t.Error("1/inf")
	}
	if d, _ := mustNew(t, "Infinity").DivE(mustNew(t, "Infinity"), 5); !d.IsNaN() {
		t.Error("inf/inf")
	}
	if d, _ := mustNew(t, "NaN").DivE(mustNew(t, "1"), 5); !d.IsNaN() {
		t.Error("nan/1")
	}
	if d := mustNew(t, "1").Div(mustNew(t, "0"), 5); !d.IsNaN() {
		t.Error("Div by zero non-error path")
	}
}

func TestDivModRemainder(t *testing.T) {
	q, r, err := mustNew(t, "13").DivMod(mustNew(t, "4"))
	if err != nil || q.String() != "0.3e1" || r.String() != "0.1e1" {
		t.Errorf("13 divmod 4 = %s,%s,%v", q, r, err)
	}
	q, r, _ = mustNew(t, "-13").DivMod(mustNew(t, "4"))
	if q.String() != "-0.4e1" || r.String() != "0.3e1" {
		t.Errorf("-13 divmod 4 = %s,%s", q, r)
	}
	m, _ := mustNew(t, "13").Mod(mustNew(t, "4"))
	if m.String() != "0.1e1" {
		t.Errorf("13 %% 4 = %s", m)
	}
	rem, _ := mustNew(t, "-13").Remainder(mustNew(t, "4"))
	if rem.String() != "-0.1e1" {
		t.Errorf("-13 remainder 4 = %s", rem)
	}
	id, _ := mustNew(t, "13").IDiv(mustNew(t, "4"))
	if id.String() != "0.3e1" {
		t.Errorf("13 div 4 = %s", id)
	}
}

func TestDivModErrors(t *testing.T) {
	if _, _, err := mustNew(t, "1").DivMod(mustNew(t, "0")); err != ErrZeroDivision {
		t.Error("divmod by 0")
	}
	if _, err := mustNew(t, "1").Mod(mustNew(t, "0")); err != ErrZeroDivision {
		t.Error("mod by 0")
	}
	if _, err := mustNew(t, "1").Remainder(mustNew(t, "0")); err != ErrZeroDivision {
		t.Error("remainder by 0")
	}
	if q, _, _ := mustNew(t, "1").DivMod(mustNew(t, "Infinity")); !q.IsNaN() {
		t.Error("divmod inf")
	}
	if q, _, _ := mustNew(t, "NaN").DivMod(mustNew(t, "1")); !q.IsNaN() {
		t.Error("divmod nan")
	}
	if r, _ := mustNew(t, "NaN").Remainder(mustNew(t, "1")); !r.IsNaN() {
		t.Error("remainder nan")
	}
}

func TestPow(t *testing.T) {
	cases := []struct {
		base string
		n    int
		want string
	}{
		{"2", 10, "0.1024e4"},
		{"2", -2, "0.25e0"},
		{"1.5", 3, "0.3375e1"},
		{"2", 0, "0.1e1"},
		{"2", 3, "0.8e1"},
		{"10", 3, "0.1e4"},
	}
	for _, c := range cases {
		if got := mustNew(t, c.base).Pow(c.n).String(); got != c.want {
			t.Errorf("%s ** %d = %s, want %s", c.base, c.n, got, c.want)
		}
	}
	if !mustNew(t, "NaN").Pow(2).IsNaN() {
		t.Error("NaN ** 2")
	}
	if mustNew(t, "0").Pow(-1).IsInfinite() != 1 {
		t.Error("0 ** -1")
	}
	if mustNew(t, "Infinity").Pow(2).IsInfinite() != 1 {
		t.Error("inf ** 2")
	}
	if mustNew(t, "-Infinity").Pow(3).IsInfinite() != -1 {
		t.Error("-inf ** 3")
	}
	if mustNew(t, "-Infinity").Pow(2).IsInfinite() != 1 {
		t.Error("-inf ** 2")
	}
	if mustNew(t, "Infinity").Pow(0).String() != "0.1e1" {
		t.Error("inf ** 0")
	}
	if !mustNew(t, "Infinity").Pow(-1).IsZero() {
		t.Error("inf ** -1")
	}
}
