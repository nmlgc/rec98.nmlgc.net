package main

import (
	"fmt"
	"html/template"
	"log"
	"math"
	"strings"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type eInvalidGame struct{}

func (e eInvalidGame) Error() string {
	return "invalid game"
}

// GameAbbrev converts a game number into an abbreviated string.
func GameAbbrev(gameNum0Based int) (string, error) {
	if gameNum0Based < 0 || gameNum0Based > 4 {
		return "", eInvalidGame{}
	}
	return fmt.Sprintf("TH%02d", gameNum0Based+1), nil
}

// ByteRange defines a range of bytes by its start and end address.
type ByteRange struct {
	Start uint
	End   uint
}

type gameComponent struct {
	binary string
	files  []string
	// Byte range occupied by the data and BSS segments of the main game code,
	// relative to the DS value used in the source. Meaning: Large number
	// after the code segment for tiny/small/compact models (where CS == DS),
	// small value for medium/large models (where DS is its own segment).
	dataRange ByteRange
}

// comp is a nice constructor for gameComponents.
func comp(binary string, dataStart uint, dataEnd uint, files ...string) gameComponent {
	return gameComponent{binary, files, ByteRange{dataStart, dataEnd}}
}

type gameSource [4]gameComponent

func (gs gameSource) Init() gameComponent      { return gs[0] }
func (gs gameSource) OP() gameComponent        { return gs[1] }
func (gs gameSource) Main() gameComponent      { return gs[2] }
func (gs gameSource) Cutscenes() gameComponent { return gs[3] }

type componentMetric [4]float64

func (cm componentMetric) Init() float64      { return cm[0] }
func (cm componentMetric) OP() float64        { return cm[1] }
func (cm componentMetric) Main() float64      { return cm[2] }
func (cm componentMetric) Cutscenes() float64 { return cm[3] }

// REMetric stores a number for each component in each binary,
// together with the per-game, per-component, and total sums.
// progress_table.html uses this structure as its data source.
type REMetric struct {
	CMetrics     [5]componentMetric // Every individual component in each game
	ComponentSum componentMetric    // All games per component
	GameSum      [5]float64         // All components per game
	Total        float64            // Everything
	// Since subtemplate calls can only take a single parameter…
	Format func(float64) template.HTML
}

// Sum updates the sums of m, based on its CMetrics.
func (m *REMetric) Sum() *REMetric {
	for i := range m.ComponentSum {
		m.ComponentSum[i] = 0
	}
	m.Total = 0
	for game, cmetric := range m.CMetrics {
		gameSum := 0.0
		for i := range cmetric {
			gameSum += cmetric[i]
			m.ComponentSum[i] += cmetric[i]
		}
		m.GameSum[game] = gameSum
		m.Total += gameSum
	}
	return m
}

// In case we'll ever need addition or subtraction... good that closures
// actually save us from needing a separate function for scalars.
func metricOp(a, b REMetric, op func(a, b float64) float64) (ret REMetric) {
	if a.Format != nil {
		ret.Format = a.Format
	} else {
		ret.Format = b.Format
	}
	for i := range a.CMetrics {
		for j := range a.ComponentSum {
			ret.CMetrics[i][j] = op(a.CMetrics[i][j], b.CMetrics[i][j])
		}
	}
	return *ret.Sum()
}

// MulBy returns the result of a multiplication of m by v as a new metric
// structure.
func (m REMetric) MulBy(v float64) (ret REMetric) {
	return metricOp(m, m, func(a, b float64) float64 { return a * v })
}

// DivByCeil returns the result of a division of m by v, followed by ceiling
// the result, as a new metric structure.
func (m REMetric) DivByCeil(v float64) (ret REMetric) {
	return metricOp(m, m, func(a, b float64) float64 {
		return math.Ceil(a / v)
	})
}

// REProgress collects all progress-indicating metrics across all of ReC98.
type REProgress struct {
	Instructions REMetric
	AbsoluteRefs REMetric
}

// REProgressPct represents the progress as percentages.
type REProgressPct REProgress

// Pct calculates the completion percentages of p relative to base.
func (p REProgress) Pct(base REProgress) (pct REProgressPct) {
	formula := func(p float64, base float64) float64 {
		return (1.0 - (p / base)) * 100.0
	}
	componentFormula := func(p componentMetric, base componentMetric) (pct componentMetric) {
		for i := range p {
			pct[i] = formula(p[i], base[i])
		}
		return
	}

	metricFormula := func(p REMetric, base REMetric) (pct REMetric) {
		for game := range p.CMetrics {
			pct.CMetrics[game] = componentFormula(p.CMetrics[game], base.CMetrics[game])
			pct.GameSum[game] = formula(p.GameSum[game], base.GameSum[game])
		}
		pct.ComponentSum = componentFormula(p.ComponentSum, base.ComponentSum)
		pct.Total = formula(p.Total, base.Total)
		pct.Format = HTMLPercentage
		return
	}

	pct.Instructions = metricFormula(p.Instructions, base.Instructions)
	pct.AbsoluteRefs = metricFormula(p.AbsoluteRefs, base.AbsoluteRefs)
	return
}

var gameSources = [5]gameSource{
	{
		comp("ZUNSOFT.COM", 0x21CE, 0x3360, "th01_zunsoft.asm"),
		comp("OP.EXE", 0x90, 0x1D2A, "th01_op.asm"),
		comp("REIIDEN.EXE", 0x90, 0x6C3A, "th01_reiiden.asm", "th01_reiiden_2.inc"),
		comp("FUUIN.EXE", 0x90, 0x1CBA, "th01_fuuin.asm"),
	}, {
		comp("ZUN.COM", 0, 0, "th02_zuninit.asm", "th02_zun_res.asm"),
		comp("OP.EXE", 0x90, 0x2340, "th02_op.asm"),
		comp("MAIN.EXE", 0x90, 0x93BA, "th02_main.asm"),
		comp("MAINE.EXE", 0x90, 0x2CE2, "th02_maine.asm"),
	}, {
		comp("ZUN.COM", 0, 0, "th03_res_yume.asm", "th03_zunsp.asm"),
		comp("OP.EXE", 0x90, 0x2510, "th03_op.asm"),
		comp("MAIN.EXE", 0x90, 0x8E90, "th03_main.asm"),
		comp("MAINL.EXE", 0x90, 0x2880, "th03_mainl.asm"),
	}, {
		comp("ZUN.COM", 0, 0, "th04_res_huma.asm"),
		comp("OP.EXE", 0x90, 0x401C, "th04_op.asm"),
		comp("MAIN.EXE", 0x90, 0xBDB2, "th04_main.asm", "th04_main_seg3+4.inc"),
		comp("MAINE.EXE", 0x90, 0x4120, "th04_maine.asm"),
	}, {
		comp("ZUN.COM", 0, 0, "th05_res_kso.asm"),
		comp("OP.EXE", 0x90, 0x51DE, "th05_op.asm"),
		comp("MAIN.EXE", 0x90, 0xC748, "th05_main.asm", "th05_main_seg3+4.inc"),
		comp("MAINE.EXE", 0x90, 0xC56E, "th05_maine.asm"),
	},
}

// REProgressAtRev goes all the way from a Git revision string to a progress
// structure.
func REProgressAtRev(rev string) (*REProgress, error) {
	commit, err := getCommit(rev)
	if err != nil {
		return nil, err
	}
	return REProgressAtCommit(commit)
}

// REProgressAtCommit retrieves the progress at the given commit.
func REProgressAtCommit(commit *object.Commit) (*REProgress, error) {
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	ret := REProgressAtTree(tree)
	return &ret, nil
}

func reProgressAtTree(tree *object.Tree) (progress REProgress) {
	type progressTuple struct {
		instructions *float64
		absoluteRefs *float64
		result       asmStats
	}
	c := make(chan progressTuple)
	filesParsed := 0

	progressFor := func(
		instructions *float64, absoluteRefs *float64, comp gameComponent,
	) {
		for _, file := range comp.files {
			f, err := tree.File(file)
			if err != nil {
				continue
			}
			fr, err := f.Reader()
			if err != nil {
				continue
			}
			go func() {
				c <- progressTuple{
					instructions, absoluteRefs,
					asmParseStats(fr, comp.dataRange),
				}
			}()
			filesParsed++
		}
	}
	for game, sources := range gameSources {
		pi := &progress.Instructions
		pr := &progress.AbsoluteRefs
		for i := range pi.ComponentSum {
			progressFor(&pi.CMetrics[game][i], &pr.CMetrics[game][i], sources[i])
		}
		pi.Format = func(val float64) template.HTML {
			return template.HTML(fmt.Sprintf("%.0f", val))
		}
		pr.Format = pi.Format
	}
	for ; filesParsed > 0; filesParsed-- {
		pt := <-c
		for _, proc := range pt.result.procs {
			*(pt.instructions) += float64(proc.instructionCount)
		}
		*(pt.absoluteRefs) += float64(pt.result.absoluteRefs)
	}

	progress.Instructions.Sum()
	progress.AbsoluteRefs.Sum()
	return
}

// REProgressAtTree parses the ASM dump files for every game at the given Git
// tree, and returns the progress for each.
var REProgressAtTree = func() func(tree *object.Tree) (progress REProgress) {
	cache := make(map[plumbing.Hash]*REProgress)
	return func(tree *object.Tree) REProgress {
		if progress, ok := cache[tree.Hash]; ok {
			return *progress
		}
		progress := reProgressAtTree(tree)
		cache[tree.Hash] = &progress
		return progress
	}
}()

// REBaselineRev returns a revision of the project where the game source .ASM
// files contain 0% third-party code, and 100% of the instructions that make
// up actual game code.
func REBaselineRev() string {
	return "re-baseline"
}

// REProgressBaseline calculates the progress at the top of the baseline
// branch, and returns a function that can return those calculated values.
func REProgressBaseline() (func() (baseline REProgress), error) {
	rev := REBaselineRev()
	log.Printf(
		"Calculating the baseline of reverse-engineering progress, from `%s`...",
		rev,
	)
	baseline, err := REProgressAtRev(rev)
	if err != nil {
		return nil, err
	}
	return func() REProgress { return *baseline }, nil
}

// RESpeed represents the total reverse-engineering speed in instructions and
// references per unit of time.
type RESpeed struct {
	Instructions float64
	AbsoluteRefs float64
}

func reSpeedPerPushFrom(diffs []DiffInfoWeighted) (spp RESpeed) {
	for _, diff := range diffs {
		top, bottom := diff.Range()
		tp, err := REProgressAtCommit(top)
		FatalIf(err)
		bp, err := REProgressAtCommit(bottom)
		FatalIf(err)

		// Yup, one value for all games, despite uth05win...
		diffInstructions := bp.Instructions.Total - tp.Instructions.Total
		diffInstructions /= diff.Pushes
		diffAbsoluteRefs := bp.AbsoluteRefs.Total - tp.AbsoluteRefs.Total
		diffAbsoluteRefs /= diff.Pushes

		spp.Instructions += diffInstructions
		spp.AbsoluteRefs += diffAbsoluteRefs
	}
	spp.Instructions /= float64(len(diffs))
	spp.AbsoluteRefs /= float64(len(diffs))
	return
}

// RESpeedPerPushFrom calculates the reverse-engineering speed based on the
// given diffs, and caches the result.
func RESpeedPerPushFrom(diffs []DiffInfoWeighted) func() RESpeed {
	log.Printf("Calculating the crowdfunding target...")
	spp := reSpeedPerPushFrom(DiffsForEstimate())
	log.Printf("Done!")
	return func() RESpeed {
		return spp
	}
}

// REProgressEstimate combines per-component completion percentages with
// per-component cost estimations for completing the rest.
type REProgressEstimate struct {
	Done  REProgressPct
	Money REProgress
}

// spp is a separate parameter because the same template that calls this
// function really should also report it to the reader. So you'd have to
// retrieve it anyway.
func reProgressEstimateAtTree(tree *object.Tree, spp RESpeed, baseline REProgress) REProgressEstimate {
	price := pushprices.Current()
	done := REProgressAtTree(tree)
	return REProgressEstimate{
		Done: done.Pct(baseline),
		Money: REProgress{
			done.Instructions.DivByCeil(spp.Instructions).MulBy(price),
			done.AbsoluteRefs.DivByCeil(spp.AbsoluteRefs).MulBy(price),
		},
	}
}

// REProgressEstimateAtTree returns a closure that returns the progress
// estimate, relative to the given baseline.
func REProgressEstimateAtTree(baseline REProgress) func(*object.Tree, RESpeed) REProgressEstimate {
	return func(tree *object.Tree, speed RESpeed) REProgressEstimate {
		return reProgressEstimateAtTree(tree, speed, baseline)
	}
}

// REMetricEstimate contains the REProgressEstimate values for a single metric.
type REMetricEstimate struct {
	Done  REMetric
	Money REMetric
}

// ForInstructions returns e's reverse-engineering values.
func (e REProgressEstimate) ForInstructions() REMetricEstimate {
	return REMetricEstimate{
		Done:  e.Done.Instructions,
		Money: e.Money.Instructions,
	}
}

// ForAbsoluteRefs returns e's position independence values.
func (e REProgressEstimate) ForAbsoluteRefs() REMetricEstimate {
	return REMetricEstimate{
		Done:  e.Done.AbsoluteRefs,
		Money: e.Money.AbsoluteRefs,
	}
}

// REEstimate represents a single estimate line, with completion percentage
// and cost estimation.
type REEstimate struct {
	Title template.HTML
	Done  float64
	Money float64
}

// ForAll returns the estimate line for all games.
func (m REMetricEstimate) ForAll() REEstimate {
	return REEstimate{
		Title: template.HTML("All games"),
		Done:  m.Done.Total,
		Money: m.Money.Total,
	}
}

// ForGameTotal returns the estimate line for completing a single game.
func (m REMetricEstimate) ForGameTotal(game int) (*REEstimate, error) {
	gameStr, err := GameAbbrev(game)
	if err != nil {
		return nil, err
	}
	icon := HTMLEmoji(strings.ToLower(gameStr))
	return &REEstimate{
		Title: icon + "&nbsp;" + template.HTML(gameStr),
		Done:  m.Done.GameSum[game],
		Money: m.Money.GameSum[game],
	}, nil
}

// ForComponents returns estimate lines for all components of the given game.
func (m REMetricEstimate) ForComponents(game int) chan REEstimate {
	ret := make(chan REEstimate)
	go func() {
		for comp := 1; comp < 4; comp++ {
			ret <- REEstimate{
				Title: template.HTML(
					fmt.Sprintf(
						"<code>%s</code>", gameSources[game][comp].binary,
					),
				),
				Done:  m.Done.CMetrics[game][comp],
				Money: m.Money.CMetrics[game][comp],
			}
		}
		close(ret)
	}()
	return ret
}
