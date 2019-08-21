package main

import "gopkg.in/src-d/go-git.v4/plumbing/object"

type gameSource struct {
	Init      []string
	OP        []string
	Main      []string
	Cutscenes []string
}

// REProgress lists the number of not yet reverse-engineered instructions in
// each component of a game
// each component of a game.
type REProgress struct {
	Init      int
	OP        int
	Main      int
	Cutscenes int
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

// REProgressAtTree parses the ASM dump files for every game at the given Git
// tree, and returns the progress for each.
func REProgressAtTree(tree *object.Tree) (progress [5]REProgress) {
	type progressTuple struct {
		target *int
		result asmStats
	}
	c := make(chan progressTuple)
	filesParsed := 0

	progressFor := func(target *int, sources []string) {
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
		progressFor(&progress[game].Init, sources.Init)
		progressFor(&progress[game].OP, sources.OP)
		progressFor(&progress[game].Main, sources.Main)
		progressFor(&progress[game].Cutscenes, sources.Cutscenes)
	}
	for ; filesParsed > 0; filesParsed-- {
		pt := <-c
		for _, proc := range pt.result {
			*(pt.target) += proc.instructionCount
		}
	}
	return
}
