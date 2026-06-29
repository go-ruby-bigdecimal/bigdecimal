<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-bigdecimal/brand/main/social/go-ruby-bigdecimal-bigdecimal.png" alt="go-ruby-bigdecimal/bigdecimal" width="720"></p>

# bigdecimal — go-ruby-bigdecimal

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-bigdecimal.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of Ruby's
[BigDecimal](https://docs.ruby-lang.org/en/master/BigDecimal.html)** — arbitrary-precision
decimal arithmetic and MRI-byte-exact formatting. It computes the same values
MRI 4.0.5's `bigdecimal` library does (so `0.1 + 0.2` is exactly `0.3`, not the
binary-float `0.30000000000000004`) and renders them in MRI's exact notation —
including the fiddly `0.15e1` scientific `to_s` default and the `to_s("F"/"E"/"+"/" "/"N")`
format grammar — **without any Ruby runtime**.

It is the decimal backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but a
**standalone, reusable** module — a sibling of
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) (the Onigmo engine),
[go-ruby-erb](https://github.com/go-ruby-erb/erb) (the ERB compiler) and
[go-ruby-yaml](https://github.com/go-ruby-yaml/yaml) (the Psych emitter/loader).

> **What it is — and isn't.** Arbitrary-precision decimal arithmetic and the
> `to_s` notation are fully deterministic and need **no interpreter**, so they
> live here as pure Go ported from MRI 4.0.5. Binding the type into a Ruby object
> model — `Kernel#BigDecimal`, operator dispatch, coercion — is the host's job;
> this library hands back an idiomatic `*Decimal` value the host wraps.

## Features

Faithful port of BigDecimal's value model and formatting, validated against the
`ruby` binary on every supported platform:

- **Construction** from a decimal string (`BigDecimal("1.5")`, exponent / `_` /
  blank-tolerant), an `int64` / `*big.Int`, or a `float64` limited to *n*
  significant digits (`BigDecimal(0.1, 10)`), with the `WithDigits(n)` precision
  cap.
- **The three specials** — `NaN`, `+Infinity`, `-Infinity` — and their
  arithmetic (`∞+∞`, `∞−∞ → NaN`, `∞×0 → NaN`, `finite/0`, …).
- **Arbitrary-precision** add / sub / mul, and **division with an explicit
  precision** (`div(o, n)`) plus the bare `/` default-precision quotient, the
  floored `div` / `%` / `divmod` / `modulo`, the truncated `remainder`, and
  integer `**` / `power`.
- **Every rounding mode** — `ROUND_UP` / `DOWN` / `HALF_UP` / `HALF_EVEN` /
  `HALF_DOWN` / `CEILING` / `FLOOR` — at any decimal place, including MRI's quirk
  that `ROUND_UP` ignores a purely-fractional discarded tail when rounding away
  whole digits.
- **`to_s` byte-for-byte** — the canonical scientific `0.<digits>e<exp>` default,
  the plain `"F"` form, the `"+"` / space sign prefixes, and the `"N"` digit
  grouping (integer part from the right, fraction/mantissa from the left).
- **Conversions & introspection** — `to_i` / `to_f` / `to_r` / `frac` / `fix` /
  `floor(n)` / `ceil(n)` / `truncate(n)` / `round(n, mode)` / `abs` / `sign` /
  `exponent` / `precision` / `split` / `<=>` / `==`.

CGO-free, dependency-free (only `math/big` for the underlying integer math),
**100% test coverage**, `gofmt` + `go vet` clean, and green across the six 64-bit
Go targets (amd64, arm64, riscv64, loong64, ppc64le, s390x).

## Install

```sh
go get github.com/go-ruby-bigdecimal/bigdecimal
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-bigdecimal/bigdecimal"
)

func main() {
	a, _ := bigdecimal.New("0.1")
	b, _ := bigdecimal.New("0.2")
	fmt.Println(a.Add(b))           // 0.3e0   (exact — not 0.30000000000000004)
	fmt.Println(a.Add(b).ToS("F"))  // 0.3

	x, _ := bigdecimal.New("1")
	y, _ := bigdecimal.New("3")
	fmt.Println(x.Div(y, 10))       // 0.3333333333e0   (10 significant digits)

	d, _ := bigdecimal.New("1234567.891")
	fmt.Println(d.ToS("5F"))        // 12 34567.891
	fmt.Println(d.String())         // 0.1234567891e7

	n, _ := bigdecimal.New("-2.5")
	fmt.Println(n)                  // -0.25e1
	r, _ := bigdecimal.New("2.5")
	fmt.Println(r.Round(0, bigdecimal.RoundHalfEven)) // 0.2e1 (banker's: 2.5 → 2)
}
```

## API

```go
type Decimal struct { /* sign, significand, base-ten exponent, special */ }

func New(s string, opts ...Option) (*Decimal, error) // BigDecimal("1.5") / BigDecimal(s, n)
func FromInt(i int64) *Decimal
func FromBigInt(i *big.Int) *Decimal
func FromFloat(f float64, n int) (*Decimal, error)    // BigDecimal(Float, n)
func WithDigits(n int) Option                         // significant-digit cap

func (d *Decimal) Add(o *Decimal) *Decimal
func (d *Decimal) Sub(o *Decimal) *Decimal
func (d *Decimal) Mul(o *Decimal) *Decimal
func (d *Decimal) Div(o *Decimal, prec int) *Decimal  // prec sig-digits; 0 = default precision
func (d *Decimal) DivE(o *Decimal, prec int) (*Decimal, error) // ErrZeroDivision on finite/0
func (d *Decimal) IDiv(o *Decimal) (*Decimal, error)  // floored integer quotient
func (d *Decimal) DivMod(o *Decimal) (q, r *Decimal, err error)
func (d *Decimal) Mod(o *Decimal) (*Decimal, error)   // floored modulo (sign of o)
func (d *Decimal) Remainder(o *Decimal) (*Decimal, error) // truncated (sign of d)
func (d *Decimal) Pow(n int) *Decimal                 // ** / power
func (d *Decimal) Neg() *Decimal
func (d *Decimal) Abs() *Decimal

func (d *Decimal) Round(n int, mode RoundMode) *Decimal
func (d *Decimal) Floor(n int) *Decimal
func (d *Decimal) Ceil(n int) *Decimal
func (d *Decimal) Truncate(n int) *Decimal
func (d *Decimal) Frac() *Decimal
func (d *Decimal) Fix() *Decimal

func (d *Decimal) Cmp(o *Decimal) int  // -1/0/1, or -2 for NaN (host maps to nil)
func (d *Decimal) Equal(o *Decimal) bool
func (d *Decimal) Sign() int           // 0 NaN, ±1 ±0, ±2 finite, ±3 ±Inf

func (d *Decimal) IsNaN() bool
func (d *Decimal) IsInfinite() int     // +1 / -1 / 0
func (d *Decimal) IsFinite() bool
func (d *Decimal) IsZero() bool

func (d *Decimal) String() string                    // to_s default (0.15e1)
func (d *Decimal) ToS(format string) string           // to_s("F"/"E"/"+"/" "/"N")
func (d *Decimal) Int() *big.Int                       // to_i (truncated)
func (d *Decimal) Float64() float64                    // to_f
func (d *Decimal) Rat() *big.Rat                       // to_r
func (d *Decimal) Exponent() int                       // #exponent
func (d *Decimal) Precision() int                      // #precision
func (d *Decimal) SplitParts() (sign int, digits string, base, exp int) // #split

type RoundMode int
const (
	RoundUp RoundMode = iota; RoundDown; RoundHalfUp; RoundHalfEven
	RoundHalfDown; RoundCeiling; RoundFloor
)
```

## `to_s` format grammar

`ToS(fmt)` mirrors MRI's `BigDecimal#to_s(fmt)`: an optional leading `+` / space
sign prefix, an optional digit-grouping count `N`, and an `F`/`f` (plain) or
`E`/`e` / empty (scientific) selector.

| call             | result for `1234567.891` |
| ---------------- | ------------------------ |
| `ToS("")`        | `0.1234567891e7`         |
| `ToS("F")`       | `1234567.891`            |
| `ToS("+")`       | `+0.1234567891e7`        |
| `ToS(" ")`       | ` 0.1234567891e7`        |
| `ToS("5F")`      | `12 34567.891`           |
| `ToS("5E")`      | `0.12345 67891e7`        |
| `ToS("3F")`      | `1 234 567.891`          |

## Tests & coverage

The suite pairs deterministic, ruby-free tests (which alone hold coverage at
100%, so the qemu cross-arch and Windows lanes pass the gate) with a
**differential MRI oracle**: a wide corpus of constructions, arithmetic,
precision divisions, every rounding mode, `to_s` formats, specials, `split` /
`exponent` / `sign`, and `<=>` is computed here and against the system `ruby`,
asserting a byte-for-byte match. The oracle scripts `$stdout.binmode` so Windows
text-mode never pollutes the bytes, gate themselves on MRI ≥ 4.0, and skip where
`ruby` is absent.

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-bigdecimal/bigdecimal authors.
