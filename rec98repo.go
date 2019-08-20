package main

import "gopkg.in/src-d/go-git.v4/plumbing/object"

type gameSource struct {
	Init      []string
	OP        []string
	Main      []string
	Cutscenes []string
}

type gameNumbers struct {
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

func numbersAtTree(tree *object.Tree) (numbers [5]gameNumbers) {
	numberFor := func(sources []string) (number int) {
		for _, file := range sources {
			f, err := tree.File(file)
			if err == nil {
				number += int(f.Size)
			}
		}
		return
	}

	for game, sources := range gameSources {
		numbers[game].Init = numberFor(sources.Init)
		numbers[game].OP = numberFor(sources.OP)
		numbers[game].Main = numberFor(sources.Main)
		numbers[game].Cutscenes = numberFor(sources.Cutscenes)
	}
	return
}
