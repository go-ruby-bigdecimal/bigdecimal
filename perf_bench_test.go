// Copyright (c) the go-ruby-bigdecimal/bigdecimal authors
//
// SPDX-License-Identifier: BSD-3-Clause

package bigdecimal

import "testing"

// These benchmarks mirror the docs cross-runtime harness operands (the
// 60-significant-digit decimal expansions of √3 and √5) so the profile lines up
// with the published numbers. They are excluded from coverage accounting.
const (
	benchA = "1.732050807568877293527446341505872366942805253810380628055806"
	benchB = "2.236067977499789696409173668731276235440618359611525724270897"
)

func benchDec(s string) *Decimal {
	d, err := New(s)
	if err != nil {
		panic(err)
	}
	return d
}

func BenchmarkAdd(b *testing.B) {
	x := benchDec(benchA)
	y := benchDec(benchB)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkD = x.Add(y)
	}
}

func BenchmarkMul(b *testing.B) {
	x := benchDec(benchA)
	y := benchDec(benchB)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkD = x.Mul(y)
	}
}

func BenchmarkDiv(b *testing.B) {
	x := benchDec(benchA)
	y := benchDec(benchB)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkD = x.Div(y, 80)
	}
}

func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkD = benchDec(benchA)
	}
}

func BenchmarkToS(b *testing.B) {
	d := benchDec(benchA).Div(benchDec(benchB), 80)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkS = d.String()
	}
}

var (
	sinkD *Decimal
	sinkS string
)
