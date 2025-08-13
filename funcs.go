// Shared functions for Go templates.

package main

import (
	"fmt"
	"html/template"

	"github.com/lucasb-eyer/go-colorful"
)

var meterGradient = GradientTable{
	{MustParseHex("#d73414"), 0},
	{MustParseHex("#d7b614"), 50},
	{MustParseHex("#75b635"), 100},
}

// CSSMeter generates a CSS background gradient for the given meter percentage.
func CSSMeter(val float64) template.CSS {
	var middle, edge, highlight string
	h, s, l := meterGradient.GetInterpolatedColorFor(val).Hsl()

	// go-colorful does not clamp these themselves.
	s = Min(s, 1/1.2)
	l = Min(l, 1/1.6)

	middle = colorful.Hsl(h, s, l).Hex()
	edge = colorful.Hsl(h, (s * 1.1), (l * 1.3)).Hex()
	highlight = colorful.Hsl(h, (s * 1.2), (l * 1.6)).Hex()
	return template.CSS(fmt.Sprintf(
		`background: linear-gradient(%v, %v 20%%, %v 45%%, %v 55%%, %v);`,
		edge, highlight, middle, middle, edge,
	))
}

var SharedFuncs = map[string]any{
	// Arithmetic
	"pct": func(f float64) float64 { return (f * 100.0) },
	"inc": func(i int) int { return i + 1 },

	// Control flow
	"loop": func(from int, to int) chan int {
		ret := make(chan int)
		go func() {
			for i := from; i < to; i++ {
				ret <- i
			}
			close(ret)
		}()
		return ret
	},

	// Markup
	"CSS_Meter": CSSMeter,
}
