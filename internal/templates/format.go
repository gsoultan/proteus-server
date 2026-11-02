// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package templates

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

// Format decides how money, numbers and dates print. It comes from a visual
// template's theme; code templates get the defaults (en-US, USD) unless
// their layout says otherwise.
type Format struct {
	Locale   string // BCP 47 tag; see locales
	Currency string // ISO 4217 code; see currencies
}

// localeInfo is what a locale changes.
type localeInfo struct {
	thousands, decimal string
	dateLayout         string // Go layout
	symbolAfter        bool   // currency symbol after the amount, as in "1.250,00 €"
}

var locales = map[string]localeInfo{
	"en-US": {thousands: ",", decimal: ".", dateLayout: "Jan 2, 2006"},
	"en-GB": {thousands: ",", decimal: ".", dateLayout: "2 Jan 2006"},
	"id-ID": {thousands: ".", decimal: ",", dateLayout: "2 Jan 2006"},
	"de-DE": {thousands: ".", decimal: ",", dateLayout: "02.01.2006", symbolAfter: true},
	"fr-FR": {thousands: " ", decimal: ",", dateLayout: "02/01/2006", symbolAfter: true},
	"es-ES": {thousands: ".", decimal: ",", dateLayout: "02/01/2006", symbolAfter: true},
	"ja-JP": {thousands: ",", decimal: ".", dateLayout: "2006/01/02"},
}

// currencyInfo is a currency's symbol and usual number of decimals.
type currencyInfo struct {
	symbol   string
	decimals int
	space    bool // space between symbol and amount, as in "Rp 1.250.000"
}

var currencies = map[string]currencyInfo{
	"USD": {symbol: "$", decimals: 2},
	"EUR": {symbol: "€", decimals: 2, space: true},
	"GBP": {symbol: "£", decimals: 2},
	"IDR": {symbol: "Rp", decimals: 0, space: true},
	"JPY": {symbol: "¥", decimals: 0},
	"SGD": {symbol: "S$", decimals: 2},
	"MYR": {symbol: "RM", decimals: 2},
	"AUD": {symbol: "A$", decimals: 2},
	"CAD": {symbol: "C$", decimals: 2},
	"INR": {symbol: "₹", decimals: 2},
	"CHF": {symbol: "CHF", decimals: 2, space: true},
	"CNY": {symbol: "¥", decimals: 2},
}

// KnownLocale and KnownCurrency report whether a tag or code is supported.
func KnownLocale(tag string) bool    { _, ok := locales[tag]; return ok }
func KnownCurrency(code string) bool { _, ok := currencies[code]; return ok }

func (f Format) locale() localeInfo {
	if l, ok := locales[f.Locale]; ok {
		return l
	}
	return locales["en-US"]
}

func (f Format) currency(code string) currencyInfo {
	if code == "" {
		code = f.Currency
	}
	if c, ok := currencies[strings.ToUpper(code)]; ok {
		return c
	}
	if code == "" {
		return currencies["USD"]
	}
	return currencyInfo{symbol: strings.ToUpper(code), decimals: 2, space: true}
}

// Number formats v with decimals (-1 for as many as needed) and the
// locale's separators.
func (f Format) Number(v float64, decimals int) string {
	l := f.locale()
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	intPart, frac, hasFrac := strings.Cut(s, ".")
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteString(l.thousands)
		}
		b.WriteRune(r)
	}
	if hasFrac {
		return sign + b.String() + l.decimal + frac
	}
	return sign + b.String()
}

// Money formats an amount in the currency (empty for the default).
func (f Format) Money(v float64, code string) string {
	c := f.currency(code)
	n := f.Number(math.Abs(v), c.decimals)
	sep := ""
	if c.space {
		sep = " "
	}
	var s string
	if f.locale().symbolAfter {
		s = n + sep + c.symbol
	} else {
		s = c.symbol + sep + n
	}
	if v < 0 {
		return "-" + s
	}
	return s
}

// Date formats an RFC 3339 or YYYY-MM-DD value in the locale's usual
// layout, or with layout when given. Other values are returned unchanged.
func (f Format) Date(v any, layout string) string {
	s := strings.TrimSpace(fmt.Sprint(v))
	if layout == "" {
		layout = f.locale().dateLayout
	}
	for _, in := range []string{time.RFC3339Nano, time.RFC3339, time.DateOnly, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(in, s); err == nil {
			return t.Format(layout)
		}
	}
	return s
}

// Percent prints a fraction (0.11) or a number above 1 (11) as 11%.
func (f Format) Percent(v float64, decimals int) string {
	if math.Abs(v) <= 1 {
		v *= 100
	}
	return f.Number(v, decimals) + "%"
}

// formatFuncs are the helpers that depend on the Format.
func (f Format) funcs(r *run) template.FuncMap {
	num := func(v any, what string) (float64, error) {
		if v == nil {
			return 0, nil
		}
		x, ok := toFloat(v)
		if !ok {
			return 0, fmt.Errorf("%s: %v is not a number", what, v)
		}
		return x, nil
	}
	agg := func(list any, key string, what string) ([]float64, error) {
		items, ok := r.items(list)
		if !ok && list != nil {
			return nil, fmt.Errorf("%s: %T is not a list", what, list)
		}
		out := make([]float64, 0, len(items))
		for _, it := range items {
			var v any = it
			if key != "" {
				m, ok := it.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("%s: list items are not objects", what)
				}
				v = m[key]
			}
			if v == nil {
				continue
			}
			x, ok := toFloat(v)
			if !ok {
				return nil, fmt.Errorf("%s: %v is not a number", what, v)
			}
			out = append(out, x)
		}
		return out, nil
	}
	return template.FuncMap{
		// money formats an amount: {{money .total}} or {{money .total "EUR"}}.
		"money": func(v any, code ...string) (string, error) {
			x, err := num(v, "money")
			if err != nil {
				return "", err
			}
			c := ""
			if len(code) > 0 {
				c = code[0]
			}
			return f.Money(x, c), nil
		},
		// fnumber formats with the locale's separators: {{fnumber .qty}} or {{fnumber .price 2}}.
		"fnumber": func(v any, decimals ...int) (string, error) {
			x, err := num(v, "fnumber")
			if err != nil {
				return "", err
			}
			d := -1
			if len(decimals) > 0 {
				d = max(-1, min(decimals[0], 10))
			}
			return f.Number(x, d), nil
		},
		// fdate formats a date in the locale's layout: {{fdate .issued}}.
		"fdate": func(v any) string { return f.Date(v, "") },
		// percent prints 0.11 or 11 as 11%: {{percent .tax_rate}}.
		"percent": func(v any, decimals ...int) (string, error) {
			x, err := num(v, "percent")
			if err != nil {
				return "", err
			}
			d := 0
			if len(decimals) > 0 {
				d = max(0, min(decimals[0], 10))
			}
			return f.Percent(x, d), nil
		},
		// sum adds a column of a list: {{sum .items "amount"}}; {{sum .amounts}} for plain numbers.
		"sum": func(list any, key ...string) (Number, error) {
			k := ""
			if len(key) > 0 {
				k = key[0]
			}
			vals, err := agg(list, k, "sum")
			if err != nil {
				return 0, err
			}
			var t float64
			for _, v := range vals {
				t += v
			}
			return Number(t), nil
		},
		// count counts the items of a list: {{count .items}}.
		"count": func(list any) (int, error) {
			items, ok := r.items(list)
			if !ok && list != nil {
				return 0, fmt.Errorf("count: %T is not a list", list)
			}
			return len(items), nil
		},
		// avg averages a column of a list: {{avg .scores "value"}}.
		"avg": func(list any, key ...string) (Number, error) {
			k := ""
			if len(key) > 0 {
				k = key[0]
			}
			vals, err := agg(list, k, "avg")
			if err != nil {
				return 0, err
			}
			if len(vals) == 0 {
				return 0, nil
			}
			var t float64
			for _, v := range vals {
				t += v
			}
			return Number(t / float64(len(vals))), nil
		},
		// add, sub, mul, div do arithmetic on numbers: {{mul .qty .price}}.
		"add": func(a, b any) (Number, error) { return arith(a, b, "add", func(x, y float64) float64 { return x + y }) },
		"sub": func(a, b any) (Number, error) { return arith(a, b, "sub", func(x, y float64) float64 { return x - y }) },
		"mul": func(a, b any) (Number, error) { return arith(a, b, "mul", func(x, y float64) float64 { return x * y }) },
		"div": func(a, b any) (Number, error) {
			y, err := num(b, "div")
			if err != nil {
				return 0, err
			}
			if y == 0 {
				return 0, errors.New("div: division by zero")
			}
			return arith(a, b, "div", func(x, y float64) float64 { return x / y })
		},
		// round rounds to a number of decimals: {{round .total 2}}.
		"round": func(v any, decimals int) (Number, error) {
			x, err := num(v, "round")
			if err != nil {
				return 0, err
			}
			p := math.Pow(10, float64(max(0, min(decimals, 10))))
			return Number(math.Round(x*p) / p), nil
		},
		// qr returns a data: URL of a QR code for use in <img src>: {{qr .verify_url}}.
		"qr": func(v any) (template.URL, error) {
			s := fmt.Sprint(v)
			if s == "" || v == nil {
				return "", nil
			}
			if len(s) > 2000 {
				return "", errors.New("qr: value is longer than 2000 bytes")
			}
			if !r.step() {
				return "", ErrTooManySteps
			}
			png, err := qrcode.Encode(s, qrcode.Medium, 256)
			if err != nil {
				return "", fmt.Errorf("qr: %v", err)
			}
			return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(png)), nil
		},
		// num converts a value to a number for comparisons: {{if gt (num .total) 1000.0}}.
		"num": func(v any) (float64, error) { return num(v, "num") },
		// nl2br turns line breaks into <br>, for addresses: {{nl2br .address}}.
		"nl2br": func(v any) template.HTML {
			var b bytes.Buffer
			for i, line := range strings.Split(fmt.Sprint(v), "\n") {
				if i > 0 {
					b.WriteString("<br>")
				}
				template.HTMLEscape(&b, []byte(line))
			}
			return template.HTML(b.String())
		},
	}
}

func arith(a, b any, what string, op func(x, y float64) float64) (Number, error) {
	x, ok := toFloat(a)
	if !ok && a != nil {
		return 0, fmt.Errorf("%s: %v is not a number", what, a)
	}
	y, ok := toFloat(b)
	if !ok && b != nil {
		return 0, fmt.Errorf("%s: %v is not a number", what, b)
	}
	return Number(op(x, y)), nil
}
