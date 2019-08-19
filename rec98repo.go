package main

import "gopkg.in/src-d/go-git.v4/plumbing/object"

type gameSource struct {
	Init      []string
	OP        []string
	Main      []string
	Cutscenes []string
}

func (gs gameSource) All() []string {
	var ret []string
	ret = append(ret, gs.Init...)
	ret = append(ret, gs.OP...)
	ret = append(ret, gs.Main...)
	return append(ret, gs.Cutscenes...)
}

var gameSources = []gameSource{
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

func numbersOfSources(tree *object.Tree, sources []string) int64 {
	ret := int64(0)
	for _, file := range sources {
		f, err := tree.File(file)
		if err == nil {
			ret += f.Size
		}
	}
	return ret
}
