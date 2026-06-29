// Copyright (c) the go-ruby-bigdecimal/bigdecimal authors
//
// SPDX-License-Identifier: BSD-3-Clause

package bigdecimal

import (
	"math"
	"testing"
)

func TestIntFloatRat(t *testing.T) {
	if mustNew(t, "3.7").Int().Int64() != 3 {
		t.Error("to_i 3.7")
	}
	if mustNew(t, "-3.7").Int().Int64() != -3 {
		t.Error("to_i -3.7")
	}
	if mustNew(t, "100").Int().Int64() != 100 {
		t.Error("to_i 100")
	}
	if mustNew(t, "NaN").Int() != nil {
		t.Error("to_i NaN")
	}
	if math.Abs(mustNew(t, "3.14").Float64()-3.14) > 1e-12 {
		t.Error("to_f")
	}
	if !math.IsNaN(mustNew(t, "NaN").Float64()) {
		t.Error("to_f NaN")
	}
	if !math.IsInf(mustNew(t, "Infinity").Float64(), 1) {
		t.Error("to_f Inf")
	}
	if mustNew(t, "3.14").Rat().RatString() != "157/50" {
		t.Errorf("to_r = %s", mustNew(t, "3.14").Rat().RatString())
	}
	if mustNew(t, "5").Rat().RatString() != "5" {
		t.Error("to_r integer")
	}
	if mustNew(t, "NaN").Rat() != nil {
		t.Error("to_r NaN")
	}
}

func TestExponentPrecisionSign(t *testing.T) {
	if mustNew(t, "123.45").Exponent() != 3 {
		t.Error("exponent 123.45")
	}
	if mustNew(t, "0.045").Exponent() != -1 {
		t.Error("exponent 0.045")
	}
	if mustNew(t, "0").Exponent() != 0 {
		t.Error("exponent 0")
	}
	if mustNew(t, "NaN").Exponent() != 0 {
		t.Error("exponent NaN")
	}
	if mustNew(t, "123.45").Precision() != 5 {
		t.Error("precision")
	}
	if mustNew(t, "0").Precision() != 0 {
		t.Error("precision 0")
	}
	if mustNew(t, "NaN").Precision() != 0 {
		t.Error("precision NaN")
	}
	signs := map[string]int{"3.14": 2, "-3.14": -2, "0": 1, "-0": -1, "NaN": 0, "Infinity": 3, "-Infinity": -3}
	for in, want := range signs {
		if got := mustNew(t, in).Sign(); got != want {
			t.Errorf("sign(%s) = %d, want %d", in, got, want)
		}
	}
}

func TestSplitParts(t *testing.T) {
	cases := []struct {
		in     string
		sign   int
		digits string
		exp    int
	}{
		{"-123.45", -1, "12345", 3},
		{"0", 1, "0", 0},
		{"NaN", 0, "NaN", 0},
		{"Infinity", 1, "Infinity", 0},
		{"-Infinity", -1, "Infinity", 0},
		{"0.001", 1, "1", -2},
	}
	for _, c := range cases {
		s, d, b, e := mustNew(t, c.in).SplitParts()
		if s != c.sign || d != c.digits || b != 10 || e != c.exp {
			t.Errorf("split(%s) = %d,%q,%d,%d; want %d,%q,10,%d", c.in, s, d, b, e, c.sign, c.digits, c.exp)
		}
	}
}

func TestCmpEqual(t *testing.T) {
	if mustNew(t, "1").Cmp(mustNew(t, "2")) != -1 {
		t.Error("1<=>2")
	}
	if mustNew(t, "2").Cmp(mustNew(t, "1")) != 1 {
		t.Error("2<=>1")
	}
	if mustNew(t, "1").Cmp(mustNew(t, "1.0")) != 0 {
		t.Error("1<=>1.0")
	}
	if mustNew(t, "NaN").Cmp(mustNew(t, "1")) != -2 {
		t.Error("NaN<=>1")
	}
	if mustNew(t, "1").Cmp(mustNew(t, "NaN")) != -2 {
		t.Error("1<=>NaN")
	}
	if mustNew(t, "Infinity").Cmp(mustNew(t, "1")) != 1 {
		t.Error("inf<=>1")
	}
	if mustNew(t, "-Infinity").Cmp(mustNew(t, "1")) != -1 {
		t.Error("-inf<=>1")
	}
	if mustNew(t, "Infinity").Cmp(mustNew(t, "Infinity")) != 0 {
		t.Error("inf<=>inf")
	}
	if mustNew(t, "Infinity").Cmp(mustNew(t, "-Infinity")) != 1 {
		t.Error("inf<=>-inf")
	}
	if mustNew(t, "-3").Cmp(mustNew(t, "-5")) != 1 {
		t.Error("-3<=>-5")
	}
	if mustNew(t, "0").Cmp(mustNew(t, "0")) != 0 {
		t.Error("0<=>0")
	}
	if mustNew(t, "0").Cmp(mustNew(t, "1")) != -1 {
		t.Error("0<=>1")
	}
	if mustNew(t, "1").Equal(mustNew(t, "1.0")) != true {
		t.Error("1==1.0")
	}
	if mustNew(t, "NaN").Equal(mustNew(t, "NaN")) != false {
		t.Error("NaN==NaN")
	}
	if mustNew(t, "1").Equal(mustNew(t, "2")) != false {
		t.Error("1==2")
	}
}

func TestPredicates(t *testing.T) {
	if !mustNew(t, "NaN").IsNaN() || mustNew(t, "1").IsNaN() {
		t.Error("IsNaN")
	}
	if mustNew(t, "Infinity").IsInfinite() != 1 || mustNew(t, "1").IsInfinite() != 0 {
		t.Error("IsInfinite")
	}
	if !mustNew(t, "1").IsFinite() || mustNew(t, "Infinity").IsFinite() || mustNew(t, "NaN").IsFinite() {
		t.Error("IsFinite")
	}
	if !mustNew(t, "0").IsZero() || mustNew(t, "1").IsZero() {
		t.Error("IsZero")
	}
}
