package main

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"regexp"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
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
type REMetric struct {
	CMetrics     [5]componentMetric // Every individual component in each game
	ComponentSum componentMetric    // All games per component
	GameSum      [5]float64         // All components per game
	Total        float64            // Everything
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

func (m REMetric) FormattedAsInt() REMetricWithFormatter {
	return REMetricWithFormatter{m, func(val float64) template.HTML {
		return template.HTML(fmt.Sprintf("%.0f", val))
	}}
}

func (m REMetric) FormattedAsPct() REMetricWithFormatter {
	return REMetricWithFormatter{m, HTMLPercentage}
}

// REMetricWithFormatter bundles a metric with a format function.
// progress_table.html uses this structure as its data source.
type REMetricWithFormatter struct {
	REMetric
	Format func(float64) template.HTML
}

// REProgress collects all progress-indicating metrics across all of ReC98.
type REProgress struct {
	CodeNotREd   REMetric
	CodeNotFinal REMetric
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
		return
	}

	pct.CodeNotREd = metricFormula(p.CodeNotREd, base.CodeNotREd)
	// Yes, not base.CodeNotFinal, since it may be larger than base.CodeNotREd.
	pct.CodeNotFinal = metricFormula(p.CodeNotFinal, base.CodeNotREd)
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
		comp("ZUN.COM", 0, 0, "th04_zuninit.asm", "th04_memchk.asm", "th04_res_huma.asm"),
		comp("OP.EXE", 0x90, 0x401C, "th04_op.asm"),
		comp("MAIN.EXE", 0x90, 0xBDB2, "th04_main.asm", "th04_main_seg3+4.inc"),
		comp("MAINE.EXE", 0x90, 0x4120, "th04_maine.asm"),
	}, {
		comp("ZUN.COM", 0, 0, "th05_zuninit.asm", "th05_gjinit.asm", "th05_memchk.asm", "th05_res_kso.asm"),
		comp("OP.EXE", 0x90, 0x51DE, "th05_op.asm"),
		comp("MAIN.EXE", 0x90, 0xC748, "th05_main.asm", "th05_main_seg3+4.inc"),
		comp("MAINE.EXE", 0x90, 0xC56E, "th05_maine.asm"),
	},
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
	type metricPointers struct {
		codeNotREd   *float64
		codeNotFinal *float64
		absoluteRefs *float64
	}
	type progressTuple struct {
		metricPointers
		result asmStats
	}
	c := make(chan progressTuple)
	filesParsed := 0
	loadMutex := sync.Mutex{}

	loadFromTree := func(fn string) (io.ReadCloser, error) {
		loadMutex.Lock()
		defer loadMutex.Unlock()
		f, err := tree.File(fn)
		if err != nil {
			return nil, err
		}
		return f.Reader()
	}

	ifNotLibOrComponent := func(fn string) bool {
		return (len(fn) >= 5 && !strings.EqualFold(fn[:5], "libs/")) &&
			!strings.EqualFold(fn, "th01_reiiden_2.inc") &&
			!strings.EqualFold(fn, "th04_main_seg3+4.inc") &&
			!strings.EqualFold(fn, "th05_main_seg3+4.inc")
	}

	progressFor := func(m metricPointers, comp gameComponent) {
		for _, fn := range comp.files {
			p := ASMParser{
				DataRange:                comp.dataRange,
				LoadFile:                 loadFromTree,
				ShouldRecurseIntoInclude: ifNotLibOrComponent,
				ProcStartMacros:          keywords{"func", "proc_defconv"},
				ProcEndMacros:            keywords{"endfunc", "endp_defconv"},
			}
			// https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
			go func(fn string) { c <- progressTuple{m, p.ParseStats(fn)} }(fn)
			filesParsed++
		}
	}
	for game, sources := range gameSources {
		for i := range progress.CodeNotREd.ComponentSum {
			m := metricPointers{
				codeNotREd:   &progress.CodeNotREd.CMetrics[game][i],
				codeNotFinal: &progress.CodeNotFinal.CMetrics[game][i],
				absoluteRefs: &progress.AbsoluteRefs.CMetrics[game][i],
			}
			progressFor(m, sources[i])
		}
	}
	for ; filesParsed > 0; filesParsed-- {
		pt := <-c
		for _, proc := range pt.result.procs {
			*(pt.codeNotREd) += float64(proc.instructionCount)
		}
		*(pt.codeNotFinal) = *(pt.codeNotREd)
		for _, proc := range pt.result.procsFromIncludes {
			*(pt.codeNotFinal) += float64(proc.instructionCount)
		}
		*(pt.absoluteRefs) += float64(pt.result.absoluteRefs)
	}

	progress.CodeNotREd.Sum()
	progress.CodeNotFinal.Sum()
	progress.AbsoluteRefs.Sum()
	return
}

// REProgressAtTree parses the ASM dump files for every game at the given Git
// tree, and returns the progress for each.
var REProgressAtTree = func() func(tree *object.Tree) (progress REProgress) {
	const CACHE_BASENAME = "progress.gob"

	type ProgressCache struct {
		ParserHash CryptHash
		RepoHash   CryptHash
		Data       map[plumbing.Hash]*REProgress
	}

	cacheNew := ProgressCache{
		ParserHash: CryptHashOfFile("asm.go"),
		RepoHash:   CryptHashOfFile("rec98repo.go"),
		Data:       make(map[plumbing.Hash]*REProgress),
	}

	cache, err := CacheLoad[ProgressCache](CACHE_BASENAME)
	if err == nil {
		tagMismatch := false
		tagMismatch = (tagMismatch || (cacheNew.ParserHash != cache.ParserHash))
		tagMismatch = (tagMismatch || (cacheNew.RepoHash != cache.RepoHash))
		if tagMismatch {
			err = errors.New("ASM parser has changed")
		}
	}
	if err != nil {
		log.Printf("Progress cache invalid (%s), will be regenerated", err)
		cache = cacheNew
	}
	return func(tree *object.Tree) REProgress {
		if progress, ok := cache.Data[tree.Hash]; ok {
			return *progress
		}
		progress := reProgressAtTree(tree)
		cache.Data[tree.Hash] = &progress
		CacheSave(CACHE_BASENAME, cache)
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
func REProgressBaseline(repo *Repository) (func() (baseline REProgress), error) {
	rev := REBaselineRev()
	log.Printf(
		"Calculating the baseline of reverse-engineering progress, from `%s`...",
		rev,
	)
	commit, err := repo.GetCommit(rev)
	if err != nil {
		return nil, err
	}
	baseline, err := REProgressAtCommit(commit)
	if err != nil {
		return nil, err
	}
	return func() REProgress { return *baseline }, nil
}

// RESpeed represents the total reverse-engineering speed in instructions and
// references per unit of time.
type RESpeed struct {
	CodeNotREd   float64
	CodeNotFinal float64
	AbsoluteRefs float64
}

func reSpeedPerPushFrom(diffs []DiffInfoWeighted) (spp RESpeed) {
	for _, diff := range diffs {
		tp, err := REProgressAtCommit(diff.Top)
		FatalIf(err)
		bp, err := REProgressAtCommit(diff.Bottom)
		FatalIf(err)

		// Yup, one value for all games, despite uth05win...
		diffCodeNotREd := bp.CodeNotREd.Total - tp.CodeNotREd.Total
		diffCodeNotREd /= diff.Pushes
		diffCodeNotFinal := bp.CodeNotFinal.Total - tp.CodeNotFinal.Total
		diffCodeNotFinal /= diff.Pushes
		diffAbsoluteRefs := bp.AbsoluteRefs.Total - tp.AbsoluteRefs.Total
		diffAbsoluteRefs /= diff.Pushes

		spp.CodeNotREd += diffCodeNotREd
		spp.CodeNotFinal += diffCodeNotFinal
		spp.AbsoluteRefs += diffAbsoluteRefs
	}
	spp.CodeNotREd /= float64(len(diffs))
	spp.CodeNotFinal /= float64(len(diffs))
	spp.AbsoluteRefs /= float64(len(diffs))
	return
}

// RESpeedPerPushFrom calculates the reverse-engineering speed based on the
// given diffs, and caches the result.
func RESpeedPerPushFrom(diffs []DiffInfoWeighted) func() RESpeed {
	log.Printf("Calculating the crowdfunding target...")
	spp := reSpeedPerPushFrom(diffs)
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
			done.CodeNotREd.DivByCeil(spp.CodeNotREd).MulBy(price),
			done.CodeNotFinal.DivByCeil(spp.CodeNotFinal).MulBy(price),
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

// ForCodeNotREd returns e's reverse-engineering values.
func (e REProgressEstimate) ForCodeNotREd() REMetricEstimate {
	return REMetricEstimate{
		Done:  e.Done.CodeNotREd,
		Money: e.Money.CodeNotREd,
	}
}

// ForCodeNotFinal returns e's finalized instruction values.
func (e REProgressEstimate) ForCodeNotFinal() REMetricEstimate {
	return REMetricEstimate{
		Done:  e.Done.CodeNotFinal,
		Money: e.Money.CodeNotFinal,
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

// AutogenerateTags adds auto-generated tags from the associated commits of a
// blog post to all entries in the given blog. Returns b itself.
func (b *Blog) AutogenerateTags(repo *Repository) *Blog {
	log.Println("Auto-generating blog post tags from associated commits…")

	var rxGames = regexp.MustCompile(` \[(th0[1-5]/?)+\]`)

	for _, entry := range b.Entries {
		var gameSeen [5]bool
		var project *ProjectInfo
		for _, p := range entry.Pushes {
			if (p.Diff.Top != nil) && (p.Diff.Bottom != nil) {
				iter, err := repo.GetLogAt(p.Diff.Top)
				FatalIf(err)
				err = iter.ForEach(func(c *object.Commit) error {
					if c.Hash == p.Diff.Bottom.Hash {
						return storer.ErrStop
					}
					subject := strings.SplitN(c.Message, "\n", 2)[0]

					if strings.HasPrefix(subject, "[Maintenance]") {
						return nil
					}
					if m := rxGames.FindString(subject); m != "" {
						for _, c := range m {
							if c >= '1' && c <= '5' {
								gameSeen[c-'1'] = true
							}
						}
					}
					return nil
				})
				FatalIf(err)
			}
			project = p.Diff.Project
		}
		for i := len(gameSeen) - 1; i >= 0; i-- {
			if gameSeen[i] {
				gameTag, _ := GameAbbrev(i)
				entry.Tags = append(
					[]string{strings.ToLower(gameTag)}, entry.Tags...,
				)
			}
		}
		if project != nil {
			entry.Tags = append(project.BlogTags, entry.Tags...)
		}
	}
	return b
}
