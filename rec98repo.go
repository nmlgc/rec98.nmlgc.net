package main

import (
	"fmt"
	"html/template"
	"log"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// ByteRange defines a range of bytes by its start and end address.
type ByteRange struct {
	Start uint
	End   uint
}

type gameComponent struct {
	files []string
	// Byte range occupied by the data and BSS segments of the main game code,
	// relative to the DS value used in  the source. Meaning: Large number
	// after the code segment for tiny/small/compact models (where CS == DS),
	// small value for medium/large models (where DS is its own segment).
	dataRange ByteRange
}

// comp is a nice constructor for gameComponents.
func comp(dataStart uint, dataEnd uint, files ...string) gameComponent {
	return gameComponent{files, ByteRange{dataStart, dataEnd}}
}

type gameSource struct {
	Init      gameComponent
	OP        gameComponent
	Main      gameComponent
	Cutscenes gameComponent
}

type componentMetric struct {
	Init      float32
	OP        float32
	Main      float32
	Cutscenes float32
}

// REMetric stores a number for each component in each binary,
// together with the per-game, per-component, and total sums.
type REMetric struct {
	CMetrics     [5]componentMetric // Every individual component in each game
	ComponentSum componentMetric    // All games per component
	GameSum      [5]float32         // All components per game
	Total        float32            // Everything
}

// Sum updates the sums of m, based on its CMetrics.
func (m *REMetric) Sum() {
	for game, cmetric := range m.CMetrics {
		gameSum := cmetric.Init + cmetric.OP + cmetric.Main + cmetric.Cutscenes
		m.ComponentSum.Init += cmetric.Init
		m.ComponentSum.OP += cmetric.OP
		m.ComponentSum.Main += cmetric.Main
		m.ComponentSum.Cutscenes += cmetric.Cutscenes
		m.GameSum[game] = gameSum
		m.Total += gameSum
	}
}

// REProgress lists the number of not yet reverse-engineered instructions in
// all of ReC98.
type REProgress = REMetric

// Format prints val as if it were an integer.
func (p REProgress) Format(val float32) string {
	return fmt.Sprintf("%.0f", val)
}

// REProgressPct represents the progress as percentages.
type REProgressPct REProgress

// Format prints val as if it were an integer.
func (p REProgressPct) Format(val float32) template.HTML {
	return template.HTML(fmt.Sprintf("%.2f&nbsp;%%", val))
}

// Pct calculates the completion percentages of p relative to base.
func (p REProgress) Pct(base REProgress) (pct REProgressPct) {
	formula := func(p float32, base float32) float32 {
		return (1.0 - (p / base)) * 100.0
	}
	componentFormula := func(p componentMetric, base componentMetric) (pct componentMetric) {
		pct.Init = formula(p.Init, base.Init)
		pct.OP = formula(p.OP, base.OP)
		pct.Main = formula(p.Main, base.Main)
		pct.Cutscenes = formula(p.Cutscenes, base.Cutscenes)
		return
	}

	for game := range p.CMetrics {
		pct.CMetrics[game] = componentFormula(p.CMetrics[game], base.CMetrics[game])
		pct.GameSum[game] = formula(p.GameSum[game], base.GameSum[game])
	}
	pct.ComponentSum = componentFormula(p.ComponentSum, base.ComponentSum)
	pct.Total = formula(p.Total, base.Total)
	return
}

var gameSources = [5]gameSource{
	{
		comp(0x21CE, 0x3360, "th01_zunsoft.asm"),
		comp(0x90, 0x1D2A, "th01_op.asm"),
		comp(0x90, 0x6C3A, "th01_reiiden.asm", "th01_reiiden_2.inc"),
		comp(0x90, 0x1CBA, "th01_fuuin.asm"),
	}, {
		comp(0, 0, "th02_zuninit.asm", "th02_zun_res.asm"),
		comp(0x90, 0x2340, "th02_op.asm"),
		comp(0x90, 0x93BA, "th02_main.asm"),
		comp(0x90, 0x2CE2, "th02_maine.asm"),
	}, {
		comp(0, 0, "th03_res_yume.asm", "th03_zunsp.asm"),
		comp(0x90, 0x2510, "th03_op.asm"),
		comp(0x90, 0x8E90, "th03_main.asm"),
		comp(0x90, 0x2880, "th03_mainl.asm"),
	}, {
		comp(0, 0, "th04_res_huma.asm"),
		comp(0x90, 0x401C, "th04_op.asm"),
		comp(0x90, 0xBDB2, "th04_main.asm", "th04_main_seg3+4.inc"),
		comp(0x90, 0x4120, "th04_maine.asm"),
	}, {
		comp(0, 0, "th05_res_kso.asm"),
		comp(0x90, 0x51DE, "th05_op.asm"),
		comp(0x90, 0xC748, "th05_main.asm", "th05_main_seg3+4.inc"),
		comp(0x90, 0xC56E, "th05_maine.asm"),
	},
}

func reProgressAtTree(tree *object.Tree) (progress REProgress) {
	type progressTuple struct {
		target *float32
		result asmStats
	}
	c := make(chan progressTuple)
	filesParsed := 0

	progressFor := func(target *float32, comp gameComponent) {
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
				c <- progressTuple{target, asmParseStats(fr)}
			}()
			filesParsed++
		}
	}
	for game, sources := range gameSources {
		progressFor(&progress.CMetrics[game].Init, sources.Init)
		progressFor(&progress.CMetrics[game].OP, sources.OP)
		progressFor(&progress.CMetrics[game].Main, sources.Main)
		progressFor(&progress.CMetrics[game].Cutscenes, sources.Cutscenes)
	}
	for ; filesParsed > 0; filesParsed-- {
		pt := <-c
		for _, proc := range pt.result.procs {
			*(pt.target) += float32(proc.instructionCount)
		}
	}

	progress.Sum()
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
func REProgressBaseline(repo *git.Repository) (func() (baseline REProgress), error) {
	rev := REBaselineRev()
	log.Printf(
		"Calculating the baseline of reverse-engineering progress, from `%s`...",
		rev,
	)
	commit, err := getCommit(rev)
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	baseline := REProgressAtTree(tree)
	return func() REProgress { return baseline }, nil
}
