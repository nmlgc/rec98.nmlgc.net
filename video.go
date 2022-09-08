package main

import "path"

type VideoRoot struct {
	Root SymmetricPath
}

func NewVideoRoot(root SymmetricPath) *VideoRoot {
	return &VideoRoot{Root: root}
}

func (r *VideoRoot) URL(stem string, codec string) string {
	return path.Join(r.Root.URLPrefix, codec, (stem + ".webm"))
}
