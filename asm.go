package main

// Cheap ASM statistics generator, because aoyud is so broken that it can't be
// effectively used to retrieve the numbers we want most… and this works just
// fine.

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// ByteRange defines a range of bytes by its start and end address.
type ByteRange struct {
	Start uint
	End   uint
}

type keywords []string

func (k keywords) match(s string) bool {
	for _, word := range k {
		if strings.EqualFold(s, word) {
			return true
		}
	}
	return false
}

var rxLabel = regexp.MustCompile(`^\s*[@\w\?]+:`)

// These come first in a line, are separated by whitespace, and may be
// followed by an argument.
var kwIgnoredInstructions = keywords{
	"nop", "include", "public", "extern", "assume", "end",

	// Since we ignore DB 0, we also must ignore these, so that one may be
	// turned into the other without "adding new instructions".
	"even", "evendata", "align",

	"if", "else", "endif", "arg", "local",
}

// These are prefixed by a symbol name, and surrounded with whitespace.
var kwIgnoredDirectives = keywords{
	"equ", "label", "segment", "struc", "ends",
}

// May appear either in "instruction" or "directive" form.
var kwData = keywords{
	"db", "dw", "dd", "dq",
}

// Special case, because it doesn't require whitespace around the = sign.
var rxEquals = regexp.MustCompile(`^[@\w]+?\s*=`)

// Yes, _TEXT catches more things than the 'CODE' class.
var rxCodeSegment = regexp.MustCompile(
	`(?i)^(?:.*_TEXT\s+segment|.+\s+segment.+'CODE'|\.code$)`,
)
var rxDataSegment = regexp.MustCompile(
	`(?i)^(?:.+?\s+segment.+'DATA'|\.data\??$)`,
)

var kwRegisters = keywords{
	"eax", "ebx", "ecx", "edx", "esp", "ebp", "esi", "edi",
	"ax", "bx", "cx", "dx", "sp", "bp", "si", "di",
	"ah", "al", "bh", "bl", "ch", "cl", "dh", "dl",
	"cs", "ds", "es", "fs", "gs", "ss",
}

var rxAddress = regexp.MustCompile(`(-?[0-9][0-9a-fA-F]{1,4})h`)

// A few instructions that can't have address immediates: INT, IN, OUT, and ENTER.
var rxNoAddressInstruction = regexp.MustCompile(`(?i)^(?:int?|out|enter)$`)

type asmProc struct {
	name             string
	instructionCount int
	hashString       string // A somewhat unique signature for this proc
}

type asmStats struct {
	procs             []asmProc
	procsFromIncludes []asmProc
	absoluteRefs      int
}

type SegmentType int

const (
	None SegmentType = iota
	Code
	Data
)

// ASMParser collects all optional parameters and state for one ASM parse run.
type ASMParser struct {
	/// Parameters
	/// ----------

	// If nonzero, the parser will count absolute memory references within this
	// range.
	DataRange ByteRange

	// Callback function for loading included files.
	LoadFile func(fn string) (io.ReadCloser, error)
	/// ----------

	ShouldRecurseIntoInclude func(fn string) bool

	// Horrible hack, but quicker than implementing macro expansion…
	ProcStartMacros keywords
	ProcEndMacros   keywords
	/// ----------

	/// State
	/// -----

	inSeg SegmentType
	/// -----
}

func (p *ASMParser) ParseStats(fn string) (ret asmStats) {
	file, err := p.LoadFile(fn)
	if err != nil {
		return ret
	}

	maybeAddress := func(s string) bool {
		addr, err := strconv.ParseInt(s, 16, 64)
		FatalIf(err)

		// Fix up negative numbers
		if addr < 0 {
			addr = 0x10000 + addr
		}
		return addr >= int64(p.DataRange.Start) &&
			addr <= int64(p.DataRange.End)
	}

	procEnter := func(name string) *asmProc {
		ret.procs = append(ret.procs, asmProc{name: name})
		return &ret.procs[len(ret.procs)-1]
	}

	procLeave := func() *asmProc {
		return nil
	}

	isCodeLine := func(line string) bool {
		if p.inSeg != Code && rxCodeSegment.MatchString(line) {
			p.inSeg = Code
			return false // Ignore *this* line
		} else if p.inSeg != Data && rxDataSegment.MatchString(line) {
			p.inSeg = Data
		}
		return p.inSeg == Code
	}

	var proc *asmProc
	scanner := bufio.NewScanner(file)
	for p.inSeg != Data && scanner.Scan() {
		line := scanner.Text()

		// Remove comments
		if i := strings.IndexByte(line, ';'); i >= 0 {
			line = line[:i]
		}

		// Ignore labels
		if label := rxLabel.FindStringIndex(line); label != nil {
			line = line[label[1]:]
		}

		if line = strings.TrimSpace(line); len(line) == 0 {
			continue
		}
		if !isCodeLine(line) {
			continue
		}

		params := strings.Fields(line)

		if len(params) > 1 {
			if len(params[1]) >= 4 {
				// Captures PROC and PROCDESC
				if strings.EqualFold(params[1][:4], "proc") {
					proc = procEnter(params[0])
					continue
				} else if strings.EqualFold(params[1], "endp") {
					proc = procLeave()
					continue
				}
			}
			if p.ProcStartMacros != nil && p.ProcStartMacros.match(params[0]) {
				proc = procEnter(params[1])
				continue
			}
			if p.ProcEndMacros != nil && p.ProcEndMacros.match(params[0]) {
				proc = procLeave()
				continue
			}

			if kwIgnoredDirectives.match(params[1]) || kwData.match(params[1]) {
				continue
			}
		}
		if strings.EqualFold(params[0], "include") &&
			p.ShouldRecurseIntoInclude(params[1]) {
			includeStats := p.ParseStats(params[1])
			ret.procsFromIncludes = append(
				ret.procsFromIncludes, includeStats.procs...,
			)
		}
		if !kwIgnoredInstructions.match(params[0]) &&
			!kwData.match(params[0]) &&
			!rxEquals.MatchString(line) {
			// OK, got an instruction that counts towards the total.

			if p.DataRange.Start > 0 {
				m := rxAddress.FindStringSubmatch(line)
				if m != nil && maybeAddress(m[1]) &&
					!rxNoAddressInstruction.MatchString(params[0]) {
					ret.absoluteRefs++
				}
			}

			if proc == nil {
				proc = procEnter(fmt.Sprintf("unnamed_%v", len(ret.procs)))
			}
			proc.hashString += params[0] + " "
			if len(params) > 1 {
				if i := strings.IndexByte(params[1], ','); i > 0 {
					params[1] = params[1][:i]
				}
				if kwRegisters.match(params[1]) {
					proc.hashString += strings.ToLower(params[1]) + " "
				}
			}
			proc.instructionCount++
		}
	}
	file.Close()
	return ret
}
