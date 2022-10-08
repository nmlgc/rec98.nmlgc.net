package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/vansante/go-ffprobe.v2"
)

// Codecs
// ------

// VideoDir defines a directory that all videos should be placed into.
type VideoDir struct {
	Dir string // under VideoRoot.Root.LocalPath
	FFMPEGCodec
}

// Lossless source files
var VIDEO_SOURCE = VideoDir{"zmbv", FFMPEGCodec{
	Ext:    ".avi",
	VCodec: "zmbv",
}}

// Best web-supported lossless codec in 2019
var VP9 = VideoDir{"vp9", FFMPEGCodec{
	Ext:    ".webm",
	VCodec: "libvpx-vp9",
}}

// Lossy fallback for outdated garbage
var VP8 = VideoDir{"vp8", FFMPEGCodec{
	Ext:    ".webm",
	VCodec: "libvpx",
}}

// VIDEO_ENCODED defines all target codec directories, ordered from the most to
// the least preferred one.
var VIDEO_ENCODED = []*VideoDir{&VP9, &VP8}

// ------

// ffmpeg
// ------

// FFMPEGCodec defines ffmpeg parameters for a codec.
type FFMPEGCodec struct {
	Ext    string // File extension
	VCodec string // -vcodec
}

// FFMPEG wraps operations that shell out to an external ffmpeg binary.
type FFMPEG struct {
	ffmpeg string
}

func NewFFMPEG() FFMPEG {
	ffmpegFN, err := exec.LookPath("ffmpeg")
	FatalIf(err)
	return FFMPEG{ffmpeg: ffmpegFN}
}

type ffmpegCodecType string

const (
	Encoder ffmpegCodecType = "-encoders"
	Decoder ffmpegCodecType = "-decoders"
)

// NewSourceMetadata runs ffprobe on the given file to generate a VideoMetadata
// struct. Only really supports AVI files, since WebM doesn't have a dedicated
// frame count field in its header.
func (f *FFMPEG) NewSourceMetadata(fn string) VideoMetadata {
	probe, err := ffprobe.ProbeURL(context.Background(), fn)
	FatalIf(err)

	videoStream := probe.FirstVideoStream()
	if videoStream == nil {
		log.Fatalf("%s: no video stream found", fn)
	}

	mustParseUint := func(s string) uint {
		ret, err := strconv.ParseUint(s, 10, 32)
		FatalIf(err)
		return uint(ret)
	}

	dividend, divisor, found := strings.Cut(videoStream.RFrameRate, "/")
	if !found {
		log.Fatalf(
			"%s: invalid frame rate format: %s", fn, videoStream.RFrameRate,
		)
	}
	fps := (float64(mustParseUint(dividend)) / float64(mustParseUint(divisor)))

	return VideoMetadata{
		FPS:        fps,
		FrameCount: mustParseUint(videoStream.NbFrames),
		Width:      uint(videoStream.Width),
		Height:     uint(videoStream.Height),
	}
}

// Supports returns the missing encoders and decoders among the given codecs
// that are not supported by this ffmpeg.
func (f FFMPEG) Supports(codecType ffmpegCodecType, codecs []*FFMPEGCodec) (missing []*FFMPEGCodec) {
	notYetSeenCount := len(codecs)
	found := make([]bool, notYetSeenCount)

	cmd := exec.Cmd{
		Path: f.ffmpeg,
		Args: []string{f.ffmpeg, "-hide_banner", string(codecType)},
	}

	// Much more reliable than reading the StdoutPipe() ourselves, as the
	// close/wait calls seem to have different behavior on Windows and Linux…
	stdout, err := cmd.Output()
	FatalIf(err)
	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	for scanner.Scan() && (notYetSeenCount > 0) {
		line := strings.TrimPrefix(scanner.Text()[len(" ------"):], " ")
		for i := range codecs {
			if !found[i] && strings.HasPrefix(line, codecs[i].VCodec) {
				found[i] = true
				notYetSeenCount--
			}
		}
	}
	for i := range found {
		if !found[i] {
			missing = append(missing, codecs[i])
		}
	}
	return
}

// ------

// VideoMetadata bundles all relevant metadata of a video.
type VideoMetadata struct {
	FPS        float64
	FrameCount uint
	Width      uint
	Height     uint
}

// Videos maps a video stem to its metadata.
type Videos map[string]VideoMetadata

type VideoRoot struct {
	Root   SymmetricPath
	ffmpeg FFMPEG
	Videos
}

func NewVideoRoot(root SymmetricPath) *VideoRoot {
	ffmpeg := NewFFMPEG()

	missing := append(
		ffmpeg.Supports(Decoder, []*FFMPEGCodec{&VIDEO_SOURCE.FFMPEGCodec}),
		ffmpeg.Supports(Encoder, func() (ret []*FFMPEGCodec) {
			for _, vd := range VIDEO_ENCODED {
				ret = append(ret, &vd.FFMPEGCodec)
			}
			return
		}())...,
	)
	if len(missing) > 0 {
		err := fmt.Sprintf(
			"The ffmpeg at %s does not support some required codecs:\n",
			ffmpeg.ffmpeg,
		)
		for _, codec := range missing {
			err += fmt.Sprintf("• %s\n", codec.VCodec)
		}
		log.Fatal(err)
	}

	log.Println("Loading video metadata…")
	videos := make(Videos)
	sourcePath := filepath.Join(root.LocalPath, VIDEO_SOURCE.Dir)
	FatalIf(filepath.WalkDir(
		sourcePath, func(fn string, info fs.DirEntry, err error) error {
			FatalIf(err)
			if info.IsDir() {
				return nil
			}
			basename := info.Name()
			ext := filepath.Ext(basename)
			if !strings.EqualFold(ext, VIDEO_SOURCE.Ext) {
				return nil
			}
			stem := strings.TrimSuffix(basename, ext)

			videos[stem] = ffmpeg.NewSourceMetadata(fn)
			return nil
		},
	))
	log.Println("Video metadata loaded.")

	return &VideoRoot{Root: root, ffmpeg: ffmpeg, Videos: videos}
}

func (r *VideoRoot) URL(stem string, vd *VideoDir) string {
	return path.Join(r.Root.URLPrefix, vd.Dir, (stem + vd.Ext))
}
