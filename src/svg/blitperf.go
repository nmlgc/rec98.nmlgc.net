// SVG visualizations of blitting benchmarks
// -----------------------------------------

package svg

import (
	"fmt"
	"html/template"
)

// BlitperfCSS returns the CSS rules for blitting benchmark visualizations, which we must generate
// server-side because we can't `<link>` stylesheets inside SVGs embedded into other SVGs for
// ✨ security reasons ✨.
func BlitperfCSS(meterFG func(val float64) template.CSS) string {
	c000 := meterFG(0)
	c033 := meterFG(33)
	c066 := meterFG(66)
	c100 := meterFG(100)
	return fmt.Sprintf(`<style type="text/css"><![CDATA[
:root {
	background-color: black;
}

text {
	fill: white;
	font-size: 400px;
	font-weight: 400;
	text-anchor: middle;
}

path {
	fill: none;
	stroke: none;
	stroke-width: 30px;
	stroke-linejoin: round;
}

rect {
	width: 250px;
	height: 250px;
}

.curve {
	stroke-width: 80;
}

.grid path {
	stroke: gray;
}

.curve000 path { stroke: %v }
.curve033 path { stroke: %v }
.curve066 path { stroke: %v }
.curve100 path { stroke: %v }
.curve000 rect { fill: %v }
.curve033 rect { fill: %v }
.curve066 rect { fill: %v }
.curve100 rect { fill: %v }
]]></style>`, c000, c033, c066, c100, c000, c033, c066, c100)
}
