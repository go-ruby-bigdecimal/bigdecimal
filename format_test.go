// Copyright (c) the go-ruby-bigdecimal/bigdecimal authors
//
// SPDX-License-Identifier: BSD-3-Clause

package bigdecimal

import "testing"

func TestToSFormats(t *testing.T) {
	d := mustNew(t, "1234567.891")
	cases := map[string]string{
		"E":   "0.1234567891e7",
		"F":   "1234567.891",
		"+":   "+0.1234567891e7",
		" ":   " 0.1234567891e7",
		"+E":  "+0.1234567891e7",
		"+F":  "+1234567.891",
		" F":  " 1234567.891",
		"5F":  "12 34567.891",
		"5E":  "0.12345 67891e7",
		"+5F": "+12 34567.891",
		"3":   "0.123 456 789 1e7",
		"3F":  "1 234 567.891",
		"3E":  "0.123 456 789 1e7",
		"e":   "0.1234567891e7",
		"f":   "1234567.891",
		"x":   "0.1234567891e7",
		"F5":  "1234567.891", // digits after the letter are ignored
		"E3":  "0.1234567891e7",
		"-":   "0.1234567891e7", // '-' is not a sign flag
		"-F":  "0.1234567891e7", // '-' not consumed, so no F form
		"":    "0.1234567891e7",
	}
	for f, want := range cases {
		if got := d.ToS(f); got != want {
			t.Errorf("ToS(%q) = %q, want %q", f, got, want)
		}
	}
}

func TestToSSignAndSpecials(t *testing.T) {
	neg := mustNew(t, "-12.5")
	if neg.ToS("+") != "-0.125e2" {
		t.Error("neg with +")
	}
	if neg.ToS(" ") != "-0.125e2" {
		t.Error("neg with space")
	}
	if neg.ToS("+F") != "-12.5" {
		t.Error("neg +F")
	}
	if mustNew(t, "3").ToS("+F") != "+3.0" {
		t.Error("pos +F")
	}
	// Specials honour the prefix on the positive form only.
	if mustNew(t, "NaN").ToS("+F") != "NaN" {
		t.Error("NaN +F")
	}
	if mustNew(t, "Infinity").ToS("+") != "+Infinity" {
		t.Error("Inf +")
	}
	if mustNew(t, "Infinity").ToS(" ") != " Infinity" {
		t.Error("Inf space")
	}
	if mustNew(t, "-Infinity").ToS("F") != "-Infinity" {
		t.Error("-Inf F")
	}
	if mustNew(t, "-Infinity").ToS("+") != "-Infinity" {
		t.Error("-Inf +")
	}
}

func TestToSZeroForms(t *testing.T) {
	for _, f := range []string{"", "F", "E"} {
		if got := mustNew(t, "0").ToS(f); got != "0.0" {
			t.Errorf("zero ToS(%q) = %q", f, got)
		}
	}
	if mustNew(t, "0").ToS("+F") != "+0.0" {
		t.Error("zero +F")
	}
	if mustNew(t, "-0").ToS("F") != "-0.0" {
		t.Error("neg zero F")
	}
	if mustNew(t, "0").ToS("3F") != "0.0" {
		t.Error("zero grouped")
	}
}

func TestPlainFormShapes(t *testing.T) {
	cases := map[string]string{
		"123456789.123456789": "123456789.123456789",
		"12.5":                "12.5",
		"100":                 "100.0",
		"0.5":                 "0.5",
		"0.001":               "0.001",
		"1000":                "1000.0",
		"0.0001234":           "0.0001234",
	}
	for in, want := range cases {
		if got := mustNew(t, in).ToS("F"); got != want {
			t.Errorf("ToS(F) of %s = %q, want %q", in, got, want)
		}
	}
	// Grouping both parts.
	if got := mustNew(t, "123456789.123456789").ToS("3F"); got != "123 456 789.123 456 789" {
		t.Errorf("grouped F = %q", got)
	}
	if got := mustNew(t, "123456789.123456789").ToS("3E"); got != "0.123 456 789 123 456 789e9" {
		t.Errorf("grouped E = %q", got)
	}
}
