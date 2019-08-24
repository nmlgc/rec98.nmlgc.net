package main

import (
	"fmt"
	"html/template"
	"log"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type gameSource struct {
	Init      []string
	OP        []string
	Main      []string
	Cutscenes []string
}

type componentCounts struct {
	Init      float32
	OP        float32
	Main      float32
	Cutscenes float32
}

// REProgress lists the number of not yet reverse-engineered instructions in
// all of ReC98.
type REProgress struct {
	ICounts      [5]componentCounts // Every individual component in each game
	ComponentSum componentCounts    // All games per component
	GameSum      [5]float32         // All components per game
	Total        float32            // Everything
}

// Format prints val as if it were an integer.
func (p REProgress) Format(val float32) string {
	return fmt.Sprintf("%.0f", val)
}

func (gs gameSource) All() []string {
	var ret []string
	ret = append(ret, gs.Init...)
	ret = append(ret, gs.OP...)
	ret = append(ret, gs.Main...)
	return append(ret, gs.Cutscenes...)
}

var gameSources = [5]gameSource{
	{
		[]string{"th01_zunsoft.asm"},
		[]string{"th01_op.asm"},
		[]string{"th01_reiiden.asm", "th01_reiiden_2.inc"},
		[]string{"th01_fuuin.asm"},
	}, {
		[]string{"th02_zuninit.asm", "th02_zun_res.asm"},
		[]string{"th02_op.asm"},
		[]string{"th02_main.asm"},
		[]string{"th02_maine.asm"},
	}, {
		[]string{"th03_res_yume.asm", "th03_zunsp.asm"},
		[]string{"th03_op.asm"},
		[]string{"th03_main.asm"},
		[]string{"th03_mainl.asm"},
	}, {
		[]string{"th04_res_huma.asm"},
		[]string{"th04_op.asm"},
		[]string{"th04_main.asm", "th04_main_seg3+4.inc"},
		[]string{"th04_maine.asm"},
	}, {
		[]string{"th05_res_kso.asm"},
		[]string{"th05_op.asm"},
		[]string{"th05_main.asm", "th05_main_seg3+4.inc"},
		[]string{"th05_maine.asm"},
	},
}

func reProgressAtTree(tree *object.Tree) (progress REProgress) {
	type progressTuple struct {
		target *float32
		result asmStats
	}
	c := make(chan progressTuple)
	filesParsed := 0

	progressFor := func(target *float32, sources []string) {
		for _, file := range sources {
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
		progressFor(&progress.ICounts[game].Init, sources.Init)
		progressFor(&progress.ICounts[game].OP, sources.OP)
		progressFor(&progress.ICounts[game].Main, sources.Main)
		progressFor(&progress.ICounts[game].Cutscenes, sources.Cutscenes)
	}
	for ; filesParsed > 0; filesParsed-- {
		pt := <-c
		for _, proc := range pt.result {
			*(pt.target) += float32(proc.instructionCount)
		}
	}

	for game, icounts := range progress.ICounts {
		gameSum := icounts.Init + icounts.OP + icounts.Main + icounts.Cutscenes
		progress.ComponentSum.Init += icounts.Init
		progress.ComponentSum.OP += icounts.OP
		progress.ComponentSum.Main += icounts.Main
		progress.ComponentSum.Cutscenes += icounts.Cutscenes
		progress.GameSum[game] = gameSum
		progress.Total += gameSum
	}
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
