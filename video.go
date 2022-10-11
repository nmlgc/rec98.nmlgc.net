package main

import "path"

// Codecs
// ------

// VideoDir defines a directory that all videos should be placed into.
type VideoDir struct {
	Dir string // under VideoRoot.Root.LocalPath
	FFMPEGCodec
}

// Best web-supported lossless codec in 2019
var VP9 = VideoDir{"vp9", FFMPEGCodec{
	Ext: ".webm",
}}

// Lossy fallback for outdated garbage
var VP8 = VideoDir{"vp8", FFMPEGCodec{
	Ext: ".webm",
}}

// VIDEO_ENCODERS defines all target codecs, ordered from the most to the least
// preferred one.
var VIDEO_ENCODERS = []*VideoDir{&VP9, &VP8}

// ------

type VideoRoot struct {
	Root SymmetricPath
}

func NewVideoRoot(root SymmetricPath) *VideoRoot {
	return &VideoRoot{Root: root}
}

func (r *VideoRoot) URL(stem string, vd *VideoDir) string {
	return path.Join(r.Root.URLPrefix, vd.Dir, (stem + vd.Ext))
}

// ffmpeg
// ------

// FFMPEGCodec defines ffmpeg parameters for a codec.
type FFMPEGCodec struct {
	Ext string // File extension
}

// ------
