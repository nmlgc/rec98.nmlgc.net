package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	"gopkg.in/vansante/go-ffprobe.v2"
)

// Codecs
// ------

// VideoDir defines a directory that all videos should be placed into.
type VideoDir struct {
	Dir string // under VideoRoot.Root.LocalPath
	FFMPEGCodec
}

// Basename returns the name of stem encoded in this directory's codec,
// relative to d.
func (d *VideoDir) Basename(stem string) string {
	return (stem + d.Ext)
}

// RelativeFN returns the name of stem encoded in this directory's codec,
// relative to the local root path.
func (d *VideoDir) RelativeFN(stem string) string {
	return filepath.Join(d.Dir, d.Basename(stem))
}

// Lossless source files
var VIDEO_SOURCE = VideoDir{"zmbv", FFMPEGCodec{
	Ext:    ".avi",
	VCodec: "zmbv",
}}

// Preview images
var POSTER = VideoDir{"poster", FFMPEGCodec{
	Ext:    ".webp",
	VCodec: "libwebp",
	VFlags: []string{
		"-frames:v", "1",
		"-lossless", "1",
	},
	VFlagsHD: []string{},
}}

// Best web-supported codec in 2022
var AV1 = VideoDir{"av1", FFMPEGCodec{
	Ext:    ".webm",
	VCodec: "libaom-av1",
	VFlags: []string{
		"-crf", "1",
		"-g", "10",
	},
	VFlagsHD: nil,
}}

// Was good for visually lossless video in 2019, turns to complete trash if
// keyframes are needed
var VP9 = VideoDir{"vp9", FFMPEGCodec{
	Ext:    ".webm",
	VCodec: "libvpx-vp9",
	VFlags: []string{
		"-crf", "15",
		"-vf", "format=yuv422p",
		"-g", "20",
	},
	VFlagsHD: []string{
		"-crf", "50",
		"-g", "30",
	},
	TwoPass: true,
}}

// Lossy fallback for outdated garbage
var VP8 = VideoDir{"vp8", FFMPEGCodec{
	Ext:    ".webm",
	VCodec: "libvpx",
	VFlags: []string{
		"-qmin", "6",
		"-qmax", "6",
		"-crf", "6",
		"-b:v", "1G",
		"-g", "30",
	},
	VFlagsHD: []string{},
}}

// VIDEO_ENCODED defines all target codec directories.
var VIDEO_ENCODED = []*VideoDir{&POSTER, &AV1, &VP9, &VP8}

// VIDEO_HOSTED defines all hosted video <source> codecs, ordered from the most
// to the least preferred one.
var VIDEO_HOSTED = VIDEO_ENCODED[1:]

// ------

// ffmpeg
// ------

// FFMPEGCodec defines ffmpeg parameters for a codec.
type FFMPEGCodec struct {
	Ext     string   // File extension
	VCodec  string   // -vcodec
	VFlags  []string // Encoding settings
	TwoPass bool

	// Encoding settings for non-pixelated HD video. If nil, HD videos aren't
	// encoded for this format; if it's a valid empty array, HD videos use the
	// settings from VFlags.
	VFlagsHD []string
}

// EqualTo returns whether these codec parameters are the same as the given
// ones.
func (c *FFMPEGCodec) EqualTo(other *FFMPEGCodec) bool {
	if (c == nil) || (other == nil) {
		return false
	}
	if len(c.VFlags) != len(other.VFlags) {
		return false
	}
	for i := range c.VFlags {
		if c.VFlags[i] != other.VFlags[i] {
			return false
		}
	}
	return (c.VCodec == other.VCodec)
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

// Encode encodes sourceFN to encodedFN with the given codec and flags,
// creating any necessary directories beforehand.
func (f *FFMPEG) Encode(encodedFN string, sourceFN string, codec *FFMPEGCodec, flags []string) {
	encodedDir, _ := filepath.Split(encodedFN)
	FatalIf(os.MkdirAll(encodedDir, 0700))
	passCount := 1
	if codec.TwoPass {
		passCount = 2
	}
	for pass := 1; pass <= passCount; pass++ {
		args := []string{
			f.ffmpeg,
			"-hide_banner",
			"-y", // force overwrite
			"-i", sourceFN,
			"-vcodec", codec.VCodec,

			// Transparent according to
			// https://wiki.xiph.org/Opus_Recommended_Settings
			"-b:a", "128k",
		}
		args = append(args, flags...)
		if codec.TwoPass {
			args = append(args, "-pass", strconv.FormatInt(int64(pass), 32))
		}
		if pass == passCount {
			args = append(args, encodedFN)
		} else {
			args = append(args, "-f", "null", "-")
		}
		cmd := exec.Cmd{
			Path:   f.ffmpeg,
			Args:   args,
			Stdout: os.Stdout,

			// ffmpeg outputs mostly to stderr, as do we. Redirect it to stdout
			// for easy filtering.
			Stderr: os.Stdout,
		}
		FatalIf(cmd.Run())
		if (pass == passCount) && codec.TwoPass {
			os.Remove("ffmpeg2pass-0.log")
		}
	}
}

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

const VIDEO_CACHE_BASENAME = "videos.gob"

type cacheMiss struct {
	refName   string
	cmpName   string
	refMetric any
	cmpMetric any
}

func (r *cacheMiss) Log(verdict string) {
	if verdict != "" {
		verdict = (", " + verdict)
	}
	lenName := Max(len(r.refName), len(r.cmpName))
	log.Printf("%s: %v\n", RightPad(r.refName, lenName), r.refMetric)
	log.Printf("%s: %v%s\n", RightPad(r.cmpName, lenName), r.cmpMetric, verdict)
}

// VideoMetadata bundles all relevant metadata of a video.
type VideoMetadata struct {
	FPS        float64
	FrameCount uint
	Width      uint
	Height     uint
}

// VideoCacheEntry bundles a video's metadata with mtimes and hashes for cache
// invalidation and re-encoding.
type VideoCacheEntry struct {
	SourceMTime  time.Time
	SourceHash   CryptHash
	Metadata     VideoMetadata
	EncodedMTime map[string]time.Time // subdirectory → mtime
}

// VideoCache bundles the stem→info map with codec settings for cache
// invalidation.
type VideoCache struct {
	Tag []*VideoDir

	// Maps a video stem to its cached information. Needs to be a pointer for
	// addressability.
	Video map[string]*VideoCacheEntry
}

func loadVideoCache(localRoot string) VideoCache {
	ret, err := CacheLoad[VideoCache](VIDEO_CACHE_BASENAME)
	if err != nil {
		log.Printf("Video cache invalid (%s), will be regenerated", err)
		return VideoCache{
			Tag:   VIDEO_ENCODED,
			Video: make(map[string]*VideoCacheEntry),
		}
	}

	var misses []cacheMiss
	for _, prev := range ret.Tag {
		prevCodec := &prev.FFMPEGCodec
		curCodecIndex := slices.IndexFunc(
			VIDEO_ENCODED, func(e *VideoDir) bool { return e.Dir == prev.Dir },
		)
		var curCodec *FFMPEGCodec
		if curCodecIndex != -1 {
			curCodec = &VIDEO_ENCODED[curCodecIndex].FFMPEGCodec
		}
		if curCodec.EqualTo(prevCodec) {
			continue
		}

		// Check if we still have stale files for the old codec configuration
		for stem := range ret.Video {
			encodedFN := filepath.Join(localRoot, prev.RelativeFN(stem))
			_, err := os.Stat(encodedFN)
			if err == nil {
				miss := cacheMiss{
					refName:   fmt.Sprintf("%s (cached)", prev.Dir),
					cmpName:   fmt.Sprintf("%s (in code)", prev.Dir),
					refMetric: *prevCodec,
					cmpMetric: "n/a",
				}
				if curCodec != nil {
					miss.cmpMetric = *curCodec
				}
				misses = append(misses, miss)
				break
			}
		}
	}
	if len(misses) > 0 {
		log.Println("Some video pipeline codec configurations changed:")
		for _, miss := range misses {
			miss.Log("")
		}
		log.Println("The respective directories still contain old video files that need to be deleted to achieve consistency with this version of the pipeline.")
		log.Fatalln("(We might also just be on an earlier commit. Exiting in any case.)")
	}
	ret.Tag = VIDEO_ENCODED
	return ret
}

func (v *VideoCache) save() {
	CacheSave(VIDEO_CACHE_BASENAME, v)
}

type VideoRoot struct {
	Root   SymmetricPath
	ffmpeg FFMPEG
	Cache  VideoCache
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

	// Loading metadata
	// ----------------

	cache := loadVideoCache(root.LocalPath)
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

			entry, inCache := cache.Video[stem]

			// Invalidate if both the source file's mtime…
			fi, err := info.Info()
			FatalIf(err)
			currentMTime := fi.ModTime()
			if !inCache || !entry.SourceMTime.Equal(currentMTime) {
				// …and its hash have changed.
				currentHash := CryptHashOfFile(fn)
				if !inCache || (currentHash != entry.SourceHash) {
					cache.Video[stem] = &VideoCacheEntry{
						SourceHash:   currentHash,
						Metadata:     ffmpeg.NewSourceMetadata(fn),
						EncodedMTime: make(map[string]time.Time),
					}
				}
				// Write a potentially changed mtime separately
				cache.Video[stem].SourceMTime = currentMTime
				cache.save()
			}
			return nil
		},
	))
	log.Println("Video metadata loaded.")
	// ----------------

	ret := &VideoRoot{Root: root, ffmpeg: ffmpeg, Cache: cache}
	go func() {
		encodedCount := 0
		for stem := range ret.Cache.Video {
			encodedCount += ret.UpdateVideo(stem)
		}
		if encodedCount != 0 {
			log.Println("Initial video conversion complete.")
		}
	}()
	return ret
}

func (r *VideoRoot) URL(stem string, vd *VideoDir) *string {
	encodedFN := filepath.Join(r.Root.LocalPath, vd.RelativeFN(stem))
	if _, err := os.Stat(encodedFN); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	ret := path.Join(r.Root.URLPrefix, vd.Dir, (stem + vd.Ext))
	return &ret
}

// UpdateVideo re-encodes a video in any codecs whose files are outdated.
// Returns the amount of videos newly encoded.
func (r *VideoRoot) UpdateVideo(stem string) (encodedCount int) {
	entry := r.Cache.Video[stem]
	sourceIsHD := strings.HasSuffix(stem, ".hd")
	sourceBasename := VIDEO_SOURCE.Basename(stem)
	sourceDebugFN := VIDEO_SOURCE.RelativeFN(stem)
	sourceFN := filepath.Join(r.Root.LocalPath, sourceDebugFN)

	// Source file deleted?
	_, err := os.Stat(sourceFN)
	deleted := errors.Is(err, fs.ErrNotExist)
	if deleted {
		log.Printf("%s deleted, removing any encoded files.", sourceDebugFN)
		delete(r.Cache.Video, stem)
		r.Cache.save()
	}

	for _, vd := range VIDEO_ENCODED {
		sourceDebugFN := strings.Repeat(" ", (len(vd.Dir) + 1))
		sourceDebugFN += sourceBasename
		encodedDebugFN := vd.RelativeFN(stem)
		encodedFN := filepath.Join(r.Root.LocalPath, encodedDebugFN)

		if deleted {
			os.Remove(encodedFN)
			continue
		}

		needsReencode := func() *cacheMiss {
			if sourceIsHD && vd.VFlagsHD == nil {
				return nil
			}

			// 1) Encoded file does not exist
			encodedFI, err := os.Stat(encodedFN)
			if errors.Is(err, fs.ErrNotExist) {
				return &cacheMiss{
					refName:   sourceDebugFN,
					cmpName:   encodedDebugFN,
					refMetric: entry.SourceMTime.String(),
					cmpMetric: "n/a",
				}
			} else if err != nil {
				FatalIf(err)
			}

			cachedMTime := entry.EncodedMTime[vd.Dir]

			// 2) Encoded file is older. Note that we use the *cached* mtime
			// here, not the one from the filesystem. This way, we also detect
			// interrupted encodes here, and restart them accordingly.
			if cachedMTime.Before(entry.SourceMTime) {
				cmpMetric := cachedMTime.String()
				if cachedMTime.IsZero() {
					cmpMetric = "outdated"
				}
				return &cacheMiss{
					refName:   sourceDebugFN,
					cmpName:   encodedDebugFN,
					refMetric: entry.SourceMTime.String(),
					cmpMetric: cmpMetric,
				}
			}

			// 3) Filesystem mtime does not match recorded mtime
			if cachedMTime != encodedFI.ModTime() {
				return &cacheMiss{
					refName:   (encodedDebugFN + " (in cache)"),
					cmpName:   (encodedDebugFN + " (on disk)"),
					refMetric: cachedMTime.String(),
					cmpMetric: encodedFI.ModTime().String(),
				}
			}

			// File up to date!
			return nil
		}

		if miss := needsReencode(); miss != nil {
			miss.Log("re-encoding")
			flags := vd.VFlags
			if sourceIsHD && len(vd.VFlagsHD) > 0 {
				flags = vd.VFlagsHD
			}
			r.ffmpeg.Encode(encodedFN, sourceFN, &vd.FFMPEGCodec, flags)
			encodedFI, err := os.Stat(encodedFN)
			FatalIf(err)
			r.Cache.Video[stem].EncodedMTime[vd.Dir] = encodedFI.ModTime()
			r.Cache.save()
			encodedCount++
		}
	}
	return
}
