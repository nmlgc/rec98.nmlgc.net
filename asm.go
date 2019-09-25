package main

// Cheap ASM statistics generator, because aoyud is so broken that it can't be
// effectively used to retrieve the numbers we want most… and this works just
// fine.

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
)

type keywords []string

func (k keywords) match(s string) bool {
	for _, word := range k {
		if strings.EqualFold(s, word) {
			return true
		}
	}
	return false
}

var rxLabel = regexp.MustCompile(`^\s*[@\w]+:(?:\s+|\z)`)

// These come first in a file, are separated by whitespace, and may be
// followed by an agument.
var kwIgnoredInstructions = keywords{
	"nop", "include", "public", "extern", "assume", "end",
}

// These are prefixed by a symbol name, and surrounded with whitespace.
var kwIgnoredDirectives = keywords{
	"equ", "label", "segment", "ends",
}

// May appear either in "instruction" or "directive" form.
var kwData = keywords{
	"db", "dw", "dd", "dq",
}

// Special case, because it doesn't require whitespace around the = sign.
var rxEquals = regexp.MustCompile(`^[@\w]+?\s*=`)

// Yes, _TEXT catches more things than the 'CODE' class.
var rxCodeSegment = regexp.MustCompile(
	`(?i)^(?:.*_TEXT\s+segment|.+\s+segment.+'CODE'|\.code\s*$)`,
)
var rxDataSegment = regexp.MustCompile(
	`(?i)^(?:.+\s*segment.+'DATA'|\.data\??\s*$)`,
)
var rxRegisters = regexp.MustCompile(
	`(?i)^((e?(ax|bx|cx|dx|sp|bp|si|di))|((a|b|c|d)(h|l))|((c|d|e|f|g|s)s))(?:\s+|\z)`,
)

var rxAddress = regexp.MustCompile(`(-?[0-9][0-9a-fA-F]{1,4})h`)

// A few instructions that can't have address immediates: INT, IN, OUT, and ENTER.
var rxNoAddressInstruction = regexp.MustCompile(`(?i)^(?:int?|out|enter)$`)

type asmProc struct {
	name             string
	instructionCount int
	hashString       string // A somewhat unique signature for this proc
}

type asmStats struct {
	procs        []asmProc
	absoluteRefs int
}

func asmParseStats(file io.ReadCloser, dataRange ByteRange) (ret asmStats) {
	maybeAddress := func(s string) bool {
		addr, err := strconv.ParseInt(s, 16, 64)
		if err != nil {
			log.Fatalln(err)
		}
		// Fix up negative numbers
		if addr < 0 {
			addr = 0x10000 + addr
		}
		return addr >= int64(dataRange.Start) && addr <= int64(dataRange.End)
	}

	type SegmentType int

	const (
		None SegmentType = iota
		Code
		Data
	)

	inSeg := None
	procCount := 0
	unnamedProcName := func() string {
		return fmt.Sprintf("unnamed_%v", procCount)
	}
	procName := unnamedProcName()

	isCodeLine := func(line string) bool {
		if inSeg != Code && rxCodeSegment.MatchString(line) {
			inSeg = Code
			return false // Ignore *this* line
		} else if inSeg != Data && rxDataSegment.MatchString(line) {
			inSeg = Data
		}
		return inSeg == Code
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
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
					if procCount < len(ret.procs) {
						procCount++
					}
					procName = params[0]
					continue
				} else if strings.EqualFold(params[1], "endp") {
					procCount++
					procName = unnamedProcName()
					continue
				}
			}
			if kwIgnoredDirectives.match(params[1]) || kwData.match(params[1]) {
				continue
			}
		}
		if !kwIgnoredInstructions.match(params[0]) &&
			!kwData.match(params[0]) &&
			!rxEquals.MatchString(line) {
			// OK, got an instruction that counts towards the total.

			if dataRange.Start > 0 {
				m := rxAddress.FindStringSubmatch(line)
				if m != nil && maybeAddress(m[1]) &&
					!rxNoAddressInstruction.MatchString(params[0]) {
					ret.absoluteRefs++
				}
			}

			if procCount >= len(ret.procs) {
				ret.procs = append(ret.procs, asmProc{name: procName})
			}
			proc := &ret.procs[procCount]
			proc.hashString += params[0] + " "
			if len(params) > 1 {
				if i := strings.IndexByte(params[1], ','); i > 0 {
					params[1] = params[1][:i]
				}
				if m := rxRegisters.FindStringSubmatch(params[1]); m != nil {
					proc.hashString += m[1] + " "
				}
			}
			proc.instructionCount++
		}
	}
	file.Close()
	return ret
}
