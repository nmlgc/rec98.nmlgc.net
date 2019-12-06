package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/narqo/go-badge"
)

/// Color generation
/// ----------------
// Adapted from https://github.com/lucasb-eyer/go-colorful/blob/master/doc/gradientgen/gradientgen.go

// GradientTable contains the "keypoints" of the color gradient you want to
// generate. The position of each keypoint has to live in the range [0, 1].
type GradientTable []struct {
	Col colorful.Color
	Pos float64
}

// GetInterpolatedColorFor is the meat of the gradient computation. It returns
// a HCL blend between the two colors around `t`.
// Note: It relies heavily on the fact that the gradient keypoints are sorted.
func (g GradientTable) GetInterpolatedColorFor(t float64) colorful.Color {
	for i := 0; i < len(g)-1; i++ {
		c1 := g[i]
		c2 := g[i+1]
		if c1.Pos <= t && t <= c2.Pos {
			// We are in between c1 and c2. Go blend them!
			t := (t - c1.Pos) / (c2.Pos - c1.Pos)
			return c1.Col.BlendHcl(c2.Col, t).Clamped()
		}
	}

	// Nothing found? Means we're at (or past) the last gradient keypoint.
	return g[len(g)-1].Col
}

// MustParseHex is a very nice thing Golang forces you to do! It is necessary
// so that we can write out the literal of the color table below.
func MustParseHex(s badge.Color) colorful.Color {
	c, err := colorful.Hex(s.String())
	if err != nil {
		panic("MustParseHex: " + err.Error())
	}
	return c
}

var gradientPoints = GradientTable{
	{MustParseHex(badge.ColorRed), 0},
	{MustParseHex(badge.ColorYellow), 50},
	{MustParseHex(badge.ColorGreen), 100},
}

/// ----------------

// Badger collects all data we want to generate badges for.
type Badger struct {
	Done REProgressPct
	Fallback http.Handler
}

// BadgeContent bundles all content seen on a badge.
type BadgeContent struct {
	Subject string
	Status  float64
	Unit    string
}

// Render writes b to wr.
func (b BadgeContent) Render(wr http.ResponseWriter) {
	col := badge.Color(gradientPoints.GetInterpolatedColorFor(b.Status).Hex())

	wr.Header().Add("Content-Type", "image/svg+xml")
	badge.Render(b.Subject, fmt.Sprintf("%.0f %%", b.Status)+b.Unit, col, wr)
}

type eInvalidBadge struct{}

func (e eInvalidBadge) Error() string {
	return "invalid badge"
}

var typeMetric = map[string]func(Badger) *REMetric{
	"re": func(b Badger) *REMetric { return &b.Done.Instructions },
	"pi": func(b Badger) *REMetric { return &b.Done.AbsoluteRefs },
}

// Parse creates a badge with the given typ for the given game.
func (b Badger) Parse(typ string, game string) (*BadgeContent, error) {
	if typ == "cap" && game == "" {
		cap := CapCurrent(nil)
		return &BadgeContent{
			"4-week crowdfunding goal", cap.FracOutstanding, " reached",
		}, nil
	}

	tm, ok := typeMetric[typ]
	if !ok {
		return nil, eInvalidBadge{}
	}

	if game == "" {
		return &BadgeContent{"All games", tm(b).Total, ""}, nil
	}
	// strconv.ParseInt() wants a bit size (and supports negative numbers),
	// fmt.Sscanf() doesn't feel right either, so…
	if len(game) != 1 {
		return nil, eInvalidBadge{}
	}
	gameNum := game[0] - '1'
	subject, err := GameAbbrev(int(gameNum))
	if err != nil {
		return nil, err
	}
	return &BadgeContent{subject, tm(b).GameSum[gameNum], ""}, nil
}

func (b Badger) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	badge, err := b.Parse(vars["type"], vars["game"])
	if err != nil {
		respondWithError(wr, err)
		return
	}
	badge.Render(wr)
}
