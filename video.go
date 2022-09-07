package main

type VideoRoot struct {
}

func NewVideoRoot() *VideoRoot {
	return &VideoRoot{}
}

func (r *VideoRoot) URL(stem string) string {
	return (stem + ".webm")
}
