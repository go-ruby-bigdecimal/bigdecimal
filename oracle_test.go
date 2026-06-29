// Copyright (c) the go-ruby-bigdecimal/bigdecimal authors
//
// SPDX-License-Identifier: BSD-3-Clause

package bigdecimal

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// rubyBin locates a usable `ruby` once and gates the oracle on MRI >= 4.0 (the
// reference version for this port). The oracle tests skip themselves when ruby is
// absent (the qemu cross-arch lanes and the Windows lane) or too old, so the
// deterministic suite alone drives the 100% gate there.
func rubyBin(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping MRI oracle")
	}
	out, err := exec.Command(path, "-e", "print RUBY_VERSION").Output()
	if err != nil {
		t.Skipf("cannot read RUBY_VERSION: %v", err)
	}
	if major, _, _ := strings.Cut(string(out), "."); major != "" {
		if n, _ := strconv.Atoi(major); n < 4 {
			t.Skipf("ruby %s < 4.0; skipping MRI oracle", out)
		}
	}
	return path
}

// rubyEval runs a -rbigdecimal one-liner and returns its stdout. The script
// $stdout.binmode-s itself so Windows text-mode never pollutes the bytes (the
// go-ruby-erb lesson; this lane only runs where ruby is present, but the preamble
// keeps the bytes deterministic everywhere).
func rubyEval(t *testing.T, bin, expr string) string {
	t.Helper()
	script := "$stdout.binmode\n$stdin.binmode\n" + expr
	out, err := exec.Command(bin, "-rbigdecimal", "-e", script).CombinedOutput()
	if err != nil {
		t.Fatalf("ruby error: %v\nscript:\n%s\noutput:\n%s", err, script, out)
	}
	return string(out)
}

// goVal builds a Decimal from a literal the same way the oracle hands it to MRI.
func goVal(t *testing.T, s string) *Decimal {
	t.Helper()
	d, err := New(s)
	if err != nil {
		t.Fatalf("New(%q): %v", s, err)
	}
	return d
}

// TestOracleToS confirms the canonical to_s and the to_s(fmt) grammar match MRI
// byte-for-byte across a wide corpus — including the fiddly 0.15e1 scientific
// default and the grouping / sign-prefix forms.
func TestOracleToS(t *testing.T) {
	bin := rubyBin(t)
	values := []string{
		"1.5", "3", "0", "-0", "10", "100", "0.1", "0.001", "12.34", "-2.5",
		"1234567890", "0.0001234", "1234567.891", "-1234567.891",
		"123456789.123456789", "NaN", "Infinity", "-Infinity", "0.5", "1000",
	}
	formats := []string{"", "E", "F", "+", " ", "+F", " F", "+E", "5F", "5E", "3", "3F", "3E"}
	for _, v := range values {
		d := goVal(t, v)
		for _, f := range formats {
			want := rubyEval(t, bin, "print BigDecimal("+strconv.Quote(v)+").to_s("+strconv.Quote(f)+")")
			if got := d.ToS(f); got != want {
				t.Errorf("ToS(%q) of %s = %q, want MRI %q", f, v, got, want)
			}
		}
	}
}

// TestOracleArithmetic checks add/sub/mul against MRI across signs and specials.
func TestOracleArithmetic(t *testing.T) {
	bin := rubyBin(t)
	pairs := [][2]string{
		{"0.1", "0.2"}, {"1", "-1"}, {"12.34", "100"}, {"-2", "3"},
		{"1234.5678", "8765.4321"}, {"0.0001", "0.00002"}, {"-5.5", "-4.5"},
		{"Infinity", "1"}, {"Infinity", "-Infinity"}, {"NaN", "2"}, {"-Infinity", "2"},
	}
	ops := map[string]func(a, b *Decimal) *Decimal{
		"+": (*Decimal).Add,
		"-": (*Decimal).Sub,
		"*": (*Decimal).Mul,
	}
	for _, p := range pairs {
		a, b := goVal(t, p[0]), goVal(t, p[1])
		for sym, fn := range ops {
			want := rubyEval(t, bin, "print (BigDecimal("+strconv.Quote(p[0])+") "+sym+" BigDecimal("+strconv.Quote(p[1])+")).to_s")
			if got := fn(a, b).ToS(""); got != want {
				t.Errorf("%s %s %s = %q, want MRI %q", p[0], sym, p[1], got, want)
			}
		}
	}
}

// TestOracleDivPrecision checks div(o, prec) and the bare #/ default precision.
func TestOracleDivPrecision(t *testing.T) {
	bin := rubyBin(t)
	cases := []struct {
		a, b string
		prec int
	}{
		{"1", "3", 10}, {"2", "7", 20}, {"13", "4", 0}, {"1", "3", 0},
		{"100", "7", 0}, {"123.456", "78.9", 0}, {"1", "30000000000", 0},
		{"5", "1000", 3}, {"22", "7", 15}, {"1", "8", 0}, {"2", "3", 5},
	}
	for _, c := range cases {
		var rubyExpr string
		if c.prec == 0 {
			rubyExpr = "print (BigDecimal(" + strconv.Quote(c.a) + ") / BigDecimal(" + strconv.Quote(c.b) + ")).to_s"
		} else {
			rubyExpr = "print BigDecimal(" + strconv.Quote(c.a) + ").div(BigDecimal(" + strconv.Quote(c.b) + ")," + strconv.Itoa(c.prec) + ").to_s"
		}
		want := rubyEval(t, bin, rubyExpr)
		if got := goVal(t, c.a).Div(goVal(t, c.b), c.prec).ToS(""); got != want {
			t.Errorf("%s/%s@%d = %q, want MRI %q", c.a, c.b, c.prec, got, want)
		}
	}
}

// TestOracleDivModRemainder checks divmod / modulo / remainder against MRI.
func TestOracleDivModRemainder(t *testing.T) {
	bin := rubyBin(t)
	pairs := [][2]string{
		{"13", "4"}, {"-13", "4"}, {"13", "-4"}, {"-13", "-4"},
		{"17.5", "4.2"}, {"-17.5", "4.2"}, {"100", "7"},
	}
	for _, p := range pairs {
		a, b := goVal(t, p[0]), goVal(t, p[1])
		q, r, _ := a.DivMod(b)
		// MRI's divmod returns [Integer, BigDecimal]; the quotient prints as a plain
		// integer, so compare q.Int() against it.
		want := rubyEval(t, bin, "q,r=BigDecimal("+strconv.Quote(p[0])+").divmod(BigDecimal("+strconv.Quote(p[1])+")); print [q.to_s, r.to_s].inspect")
		got := "[" + strconv.Quote(q.Int().String()) + ", " + strconv.Quote(r.ToS("")) + "]"
		if got != want {
			t.Errorf("%s divmod %s = %s, want MRI %s", p[0], p[1], got, want)
		}
		rem, _ := a.Remainder(b)
		wantRem := rubyEval(t, bin, "print BigDecimal("+strconv.Quote(p[0])+").remainder(BigDecimal("+strconv.Quote(p[1])+")).to_s")
		if rem.ToS("") != wantRem {
			t.Errorf("%s remainder %s = %q, want MRI %q", p[0], p[1], rem.ToS(""), wantRem)
		}
	}
}

// TestOracleRounding checks every rounding mode against MRI's BigDecimal::ROUND_*.
func TestOracleRounding(t *testing.T) {
	bin := rubyBin(t)
	modes := []struct {
		mode RoundMode
		ruby string
	}{
		{RoundUp, "ROUND_UP"},
		{RoundDown, "ROUND_DOWN"},
		{RoundHalfUp, "ROUND_HALF_UP"},
		{RoundHalfEven, "ROUND_HALF_EVEN"},
		{RoundHalfDown, "ROUND_HALF_DOWN"},
		{RoundCeiling, "ROUND_CEILING"},
		{RoundFloor, "ROUND_FLOOR"},
	}
	values := []string{"2.5", "3.5", "-2.5", "2.4", "2.6", "2.55", "-2.55", "0.125", "3.14159", "123.456"}
	places := []int{0, 2, -1}
	for _, m := range modes {
		for _, v := range values {
			for _, n := range places {
				want := rubyEval(t, bin, "print BigDecimal("+strconv.Quote(v)+").round("+strconv.Itoa(n)+", BigDecimal::"+m.ruby+").to_s")
				if got := goVal(t, v).Round(n, m.mode).ToS(""); got != want {
					t.Errorf("round(%s, %d, %s) = %q, want MRI %q", v, n, m.ruby, got, want)
				}
			}
		}
	}
}

// TestOracleFloorCeilTruncate checks floor/ceil/truncate at several places.
func TestOracleFloorCeilTruncate(t *testing.T) {
	bin := rubyBin(t)
	values := []string{"3.14159", "-3.14159", "123.456", "-123.456", "0.5"}
	for _, v := range values {
		for _, n := range []int{0, 1, 2, -1, -2} {
			ns := strconv.Itoa(n)
			for _, m := range []struct {
				name string
				fn   func(*Decimal, int) *Decimal
			}{
				{"floor", (*Decimal).Floor},
				{"ceil", (*Decimal).Ceil},
				{"truncate", (*Decimal).Truncate},
			} {
				want := rubyEval(t, bin, "print BigDecimal("+strconv.Quote(v)+")."+m.name+"("+ns+").to_s")
				if got := m.fn(goVal(t, v), n).ToS(""); got != want {
					t.Errorf("%s(%s, %d) = %q, want MRI %q", m.name, v, n, got, want)
				}
			}
		}
	}
}

// TestOraclePow checks integer powers against MRI's #power.
func TestOraclePow(t *testing.T) {
	bin := rubyBin(t)
	bases := []string{"2", "1.5", "10", "0.5", "-3"}
	for _, b := range bases {
		for _, n := range []int{0, 1, 2, 3, 5, 10} {
			want := rubyEval(t, bin, "print (BigDecimal("+strconv.Quote(b)+") ** "+strconv.Itoa(n)+").to_s")
			if got := goVal(t, b).Pow(n).ToS(""); got != want {
				t.Errorf("%s ** %d = %q, want MRI %q", b, n, got, want)
			}
		}
	}
}

// TestOracleSplitExponentSign checks split / exponent / sign against MRI.
func TestOracleSplitExponentSign(t *testing.T) {
	bin := rubyBin(t)
	values := []string{"-123.45", "0", "0.001", "12345", "NaN", "Infinity", "-Infinity", "-0"}
	for _, v := range values {
		s, dig, base, exp := goVal(t, v).SplitParts()
		want := rubyEval(t, bin, "print BigDecimal("+strconv.Quote(v)+").split.inspect")
		got := "[" + strconv.Itoa(s) + ", " + strconv.Quote(dig) + ", " + strconv.Itoa(base) + ", " + strconv.Itoa(exp) + "]"
		if got != want {
			t.Errorf("split(%s) = %s, want MRI %s", v, got, want)
		}
		wantExp := rubyEval(t, bin, "print BigDecimal("+strconv.Quote(v)+").exponent")
		if got := strconv.Itoa(goVal(t, v).Exponent()); got != wantExp {
			t.Errorf("exponent(%s) = %s, want MRI %s", v, got, wantExp)
		}
		wantSign := rubyEval(t, bin, "print BigDecimal("+strconv.Quote(v)+").sign")
		if got := strconv.Itoa(goVal(t, v).Sign()); got != wantSign {
			t.Errorf("sign(%s) = %s, want MRI %s", v, got, wantSign)
		}
	}
}

// TestOracleConversions checks to_i / to_f / to_r against MRI.
func TestOracleConversions(t *testing.T) {
	bin := rubyBin(t)
	values := []string{"3.7", "-3.7", "100", "3.14", "-3.14", "0.25"}
	for _, v := range values {
		wantI := rubyEval(t, bin, "print BigDecimal("+strconv.Quote(v)+").to_i.to_s")
		if got := goVal(t, v).Int().String(); got != wantI {
			t.Errorf("to_i(%s) = %s, want MRI %s", v, got, wantI)
		}
		wantR := rubyEval(t, bin, "r=BigDecimal("+strconv.Quote(v)+").to_r; print \"#{r.numerator}/#{r.denominator}\"")
		got := goVal(t, v).Rat()
		if rs := got.Num().String() + "/" + got.Denom().String(); rs != wantR {
			t.Errorf("to_r(%s) = %s, want MRI %s", v, rs, wantR)
		}
	}
}

// TestOracleCompare checks <=> and == against MRI.
func TestOracleCompare(t *testing.T) {
	bin := rubyBin(t)
	pairs := [][2]string{
		{"1", "2"}, {"2", "1"}, {"1", "1.0"}, {"-3", "-5"}, {"0", "0"},
		{"Infinity", "1"}, {"-Infinity", "1"}, {"Infinity", "Infinity"},
		{"NaN", "1"}, {"1", "NaN"},
	}
	for _, p := range pairs {
		got := goVal(t, p[0]).Cmp(goVal(t, p[1]))
		want := rubyEval(t, bin, "print (BigDecimal("+strconv.Quote(p[0])+") <=> BigDecimal("+strconv.Quote(p[1])+")).inspect")
		var gotStr string
		if got == -2 {
			gotStr = "nil"
		} else {
			gotStr = strconv.Itoa(got)
		}
		if gotStr != want {
			t.Errorf("%s <=> %s = %s, want MRI %s", p[0], p[1], gotStr, want)
		}
	}
}
