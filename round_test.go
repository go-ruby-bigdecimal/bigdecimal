// Copyright (c) the go-ruby-bigdecimal/bigdecimal authors
//
// SPDX-License-Identifier: BSD-3-Clause

package bigdecimal

import "testing"

// roundModeTable holds the MRI round(0, mode) results for a fixed value set,
// captured from `ruby -rbigdecimal`.
func TestRoundModes(t *testing.T) {
	vals := []string{"2.5", "3.5", "-2.5", "2.4", "2.6", "2.55", "-2.55", "0.125", "0.135"}
	type row struct {
		mode RoundMode
		want []string
	}
	rows := []row{
		{RoundUp, []string{"3.0", "4.0", "-3.0", "3.0", "3.0", "3.0", "-3.0", "1.0", "1.0"}},
		{RoundDown, []string{"2.0", "3.0", "-2.0", "2.0", "2.0", "2.0", "-2.0", "0.0", "0.0"}},
		{RoundHalfUp, []string{"3.0", "4.0", "-3.0", "2.0", "3.0", "3.0", "-3.0", "0.0", "0.0"}},
		{RoundHalfEven, []string{"2.0", "4.0", "-2.0", "2.0", "3.0", "3.0", "-3.0", "0.0", "0.0"}},
		{RoundHalfDown, []string{"2.0", "3.0", "-2.0", "2.0", "3.0", "3.0", "-3.0", "0.0", "0.0"}},
		{RoundCeiling, []string{"3.0", "4.0", "-2.0", "3.0", "3.0", "3.0", "-2.0", "1.0", "1.0"}},
		{RoundFloor, []string{"2.0", "3.0", "-3.0", "2.0", "2.0", "2.0", "-3.0", "0.0", "0.0"}},
	}
	for _, r := range rows {
		for i, v := range vals {
			got := mustNew(t, v).Round(0, r.mode).ToS("F")
			if got != r.want[i] {
				t.Errorf("round(%s, mode %d) = %s, want %s", v, r.mode, got, r.want[i])
			}
		}
	}
}

func TestRoundFloorCeilTruncateN(t *testing.T) {
	d := mustNew(t, "3.14159")
	if d.Round(2, RoundHalfUp).ToS("F") != "3.14" {
		t.Error("round 2")
	}
	if d.Round(2, RoundDown).ToS("F") != "3.14" {
		t.Error("round 2 down")
	}
	if d.Floor(2).ToS("F") != "3.14" {
		t.Error("floor 2")
	}
	if d.Ceil(2).ToS("F") != "3.15" {
		t.Error("ceil 2")
	}
	if d.Truncate(2).ToS("F") != "3.14" {
		t.Error("truncate 2")
	}
	if d.Floor(0).ToS("F") != "3.0" {
		t.Error("floor 0")
	}
	if d.Round(0, RoundHalfUp).ToS("F") != "3.0" {
		t.Error("round 0")
	}
	if mustNew(t, "-3.7").Floor(0).ToS("F") != "-4.0" {
		t.Error("floor -3.7")
	}
	if mustNew(t, "-3.2").Ceil(0).ToS("F") != "-3.0" {
		t.Error("ceil -3.2")
	}
	// Negative places.
	if mustNew(t, "123.456").Round(-1, RoundHalfUp).ToS("F") != "120.0" {
		t.Errorf("round -1 = %s", mustNew(t, "123.456").Round(-1, RoundHalfUp).ToS("F"))
	}
	if mustNew(t, "123.456").Floor(-2).ToS("F") != "100.0" {
		t.Error("floor -2")
	}
	if mustNew(t, "125.456").Round(-1, RoundHalfUp).ToS("F") != "130.0" {
		t.Error("round -1 up")
	}
	// Rounding a value already coarser than the place is a no-op.
	if mustNew(t, "100").Round(2, RoundHalfUp).ToS("F") != "100.0" {
		t.Error("round coarse")
	}
	// Rounding a zero.
	if mustNew(t, "0").Round(2, RoundHalfUp).ToS("F") != "0.0" {
		t.Error("round zero")
	}
	// Rounding specials.
	if !mustNew(t, "NaN").Round(2, RoundHalfUp).IsNaN() {
		t.Error("round NaN")
	}
	if mustNew(t, "Infinity").Round(2, RoundHalfUp).IsInfinite() != 1 {
		t.Error("round Inf")
	}
}

// TestRoundUpNegativePlaceQuirk pins MRI's behaviour that ROUND_UP/DOWN ignore a
// purely-fractional discarded remainder when rounding away whole digits, while
// CEILING and the HALF modes still see it. Values captured from MRI 4.0.
func TestRoundUpNegativePlaceQuirk(t *testing.T) {
	// ROUND_UP at the tens place (n = -1).
	upTens := map[string]string{
		"0.99": "0.0",   // sub-unit fraction → not rounded away from zero
		"0.5":  "0.0",   // ditto
		"1":    "0.1e2", // an integer digit is discarded → up to 10
		"9":    "0.1e2", // up to 10
		"9.9":  "0.1e2", // integer digit '9' present → up
	}
	for in, want := range upTens {
		if got := mustNew(t, in).Round(-1, RoundUp).String(); got != want {
			t.Errorf("round(%s, -1, UP) = %s, want %s", in, got, want)
		}
	}
	// At the hundreds place (n = -2) the boundary is one whole unit.
	if got := mustNew(t, "9.9").Round(-2, RoundUp).String(); got != "0.1e3" {
		t.Errorf("round(9.9, -2, UP) = %s", got)
	}
	if got := mustNew(t, "0.5").Round(-2, RoundUp).String(); got != "0.0" {
		t.Errorf("round(0.5, -2, UP) = %s", got)
	}
	// CEILING does honour the fraction (rounds toward +∞).
	if got := mustNew(t, "0.99").Round(-1, RoundCeiling).String(); got != "0.1e2" {
		t.Errorf("round(0.99, -1, CEILING) = %s", got)
	}
	// HALF_EVEN sees the fraction breaking a tie: 25.5 → 30, 25 → 20.
	if got := mustNew(t, "25.5").Round(-1, RoundHalfEven).String(); got != "0.3e2" {
		t.Errorf("round(25.5, -1, HALF_EVEN) = %s", got)
	}
	if got := mustNew(t, "25").Round(-1, RoundHalfEven).String(); got != "0.2e2" {
		t.Errorf("round(25, -1, HALF_EVEN) = %s", got)
	}
	// ROUND_UP at a fractional place still honours the whole tail.
	if got := mustNew(t, "0.1009").Round(2, RoundUp).String(); got != "0.11e0" {
		t.Errorf("round(0.1009, 2, UP) = %s", got)
	}
	// An integer value (exp >= 0) discarding whole digits hits the exp>=0 branch.
	if got := mustNew(t, "1200").Round(-3, RoundUp).String(); got != "0.2e4" {
		t.Errorf("round(1200, -3, UP) = %s", got)
	}
}

func TestFracFix(t *testing.T) {
	if mustNew(t, "3.14").Frac().String() != "0.14e0" {
		t.Errorf("frac = %s", mustNew(t, "3.14").Frac().String())
	}
	if mustNew(t, "3.14").Fix().String() != "0.3e1" {
		t.Errorf("fix = %s", mustNew(t, "3.14").Fix().String())
	}
	if !mustNew(t, "5").Frac().IsZero() {
		t.Error("frac of integer")
	}
	if !mustNew(t, "NaN").Frac().IsNaN() {
		t.Error("frac NaN")
	}
	if mustNew(t, "Infinity").Fix().IsInfinite() != 1 {
		t.Error("fix Inf")
	}
	if !mustNew(t, "NaN").Fix().IsNaN() {
		t.Error("fix NaN")
	}
	if mustNew(t, "Infinity").Frac().IsInfinite() != 1 {
		t.Error("frac Inf")
	}
}
