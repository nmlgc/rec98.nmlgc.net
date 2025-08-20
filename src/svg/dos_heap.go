// SVG visualizations of the DOS heap
// ----------------------------------

package svg

import (
	"fmt"
	"html/template"
	"strings"
)

var TEXT_CENTER = 30.0
var TEXT_LINE_HEIGHT = 12.5

var COLOR_GAME = template.HTMLAttr(`#f88`)

// MCB represents a single DOS Memory Control Block within a SVG.
type MCB struct {
	name string
	L    float64 // Left coordinate
	R    float64 // Right coordinate
	W    float64 // Width
	CX   float64 // Horizontal center
}

func (m *MCB) Path() string {
	return fmt.Sprintf(`d="M%v 0h %vv 60H %vz"`, m.L, m.W, m.L)
}

func (m *MCB) Name() string {
	lines := strings.Split(m.name, "\n")
	top := TEXT_CENTER - (float64(len(lines)-1) * (TEXT_LINE_HEIGHT / 2.0))
	ret := fmt.Sprintf(`<text class="label" x="%v" y="%v">`, m.CX, top)
	for i, line := range lines {
		if i == 0 {
			ret += (`<tspan>` + line + `</tspan>`)
		} else {
			ret += fmt.Sprintf(`<tspan x="%v" dy="%v">%s</tspan>`, m.CX, TEXT_LINE_HEIGHT, line)
		}
	}
	ret += `</text>`
	return ret
}

func (m *MCB) RegularAlloc() string {
	return fmt.Sprintf(`<path class="alloc" %v/>%v`, m.Path(), m.Name())
}

func (m *MCB) Game() string {
	return fmt.Sprintf(
		`<path class="alloc" style="fill: %v" %v/>%v`, COLOR_GAME, m.Path(), m.Name(),
	)
}

func (m *MCB) Free() string {
	return fmt.Sprintf(
		`<g class="free">
			<line class="alloc" x1="%v" y1="0" x2="%v" y2="0" stroke="#000" />%v
		</g>`,
		m.L, m.R, m.Name())
}

// Heap represents an instance of a DOS heap.
type Heap struct {
	cur float64
}

func NewHeap() *Heap {
	return &Heap{cur: 0}
}

func (h *Heap) CSS() string {
	return `<link xmlns="http://www.w3.org/1999/xhtml" rel="stylesheet" href="/static/heap.css" type="text/css" />`
}

// SysGroup wraps body into system-block formatting.
func (h *Heap) SysGroup(body string) string {
	return `<g opacity="0.25">` + body + `</g>`
}

// BIOSAndDOS renders the BIOS and COMMAND.COM MCBs.
func (h *Heap) BIOSAndDOS(bios *MCB, dos *MCB) string {
	return h.SysGroup(fmt.Sprintf(
		`<path class="alloc fixed" %v/>%v%v`,
		bios.Path(), bios.Name(), dos.RegularAlloc()))
}

func (h *Heap) TRAM() string {
	tram := (&Heap{cur: 640}).MCB("Text RAM", 60)
	return h.SysGroup(fmt.Sprintf(`<path class="alloc fixed" %v/>%v`, tram.Path(), tram.Name()))
}

// Line renders the number line and legend.
func (h *Heap) Line() string {
	return `
	<text class="number" x="0.5" y="77.5" style="text-anchor: start;">0000h</text>
	<text class="number" x="639.5" y="77.5" style="text-anchor: end;">9FFFh</text>
	<text class="number" x="645" y="77.5" style="text-anchor: start;">(640 KiB)</text>
	<path d="M0 60h700" stroke="#000"/>
	<path d="M0 55v10" stroke="#000"/>
	<path d="M640 55v10" stroke="#000"/>`
}

// MCB creates a new MCB at the current end of the heap.
func (h *Heap) MCB(name string, w float64) *MCB {
	ret := &MCB{
		name: name,
		L:    h.cur,
		R:    (h.cur + w),
		W:    w,
		CX:   (h.cur + (w / 2)),
	}
	h.cur += w
	return ret
}
