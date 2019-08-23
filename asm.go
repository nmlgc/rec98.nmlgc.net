package main

// Cheap ASM statistics generator, because aoyud is so broken that it can't be
// effectively used to retrieve the numbers we want most… and this works just
// fine.

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var rxLabel = regexp.MustCompile(`\s*\w+:(\s+|\z)`)
var rxProcStart = regexp.MustCompile(`(?i)(.+?)\s+proc`)
var rxProcEnd = regexp.MustCompile(`(?i)(.+?)\s+endp`)
var rxIgnoredInstructions = regexp.MustCompile(
	`(?i)\b(nop|db|dw|dd|dq|include|public|extern|assume|end)\b`,
)
var rxIgnoredDirectives = regexp.MustCompile(
	`(?i)(.+)\s*(\=|equ|label|ends)(\s+|\z)`,
)

// Yes, _TEXT catches more things than the 'CODE' class.
var rxCodeSegment = regexp.MustCompile(
	`(?i)^((.*_TEXT)\s+segment)|.+\s+segment.+'CODE'|\.code\s*$`,
)
var rxDataSegment = regexp.MustCompile(
	`(?i)^((.+)\s*segment.+'DATA')|\.data\??\s*$`,
)
var rxRegisters = regexp.MustCompile(
	`(?i)^((e?(ax|bx|cx|dx|sp|bp|si|di))|((a|b|c|d)(h|l))|((c|d|e|f|g|s)s))(\s+|\z)`,
)

type asmProc struct {
	name             string
	instructionCount int
	hashString       string // A somewhat unique signature for this proc
}

type asmStats []asmProc

func asmParseStats(file io.ReadCloser) asmStats {
	type SegmentType int

	const (
		None SegmentType = iota
		Code
		Data
	)

	var ret []asmProc
	inSeg := None
	procCount := 0
	unnamedProcName := func() string {
		return fmt.Sprintf("unnamed_%v", procCount)
	}
	procName := unnamedProcName()

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

		if rxCodeSegment.MatchString(line) {
			inSeg = Code
		} else if rxDataSegment.MatchString(line) {
			inSeg = Data
		} else if m := rxProcStart.FindStringSubmatch(line); m != nil {
			if procCount < len(ret) {
				procCount++
			}
			procName = m[1]
		} else if rxProcEnd.MatchString(line) {
			procCount++
			procName = unnamedProcName()
		} else if inSeg == Code &&
			!rxIgnoredInstructions.MatchString(line) &&
			!rxIgnoredDirectives.MatchString(line) {
			// OK, got an instruction that counts towards the total.
			params := strings.Fields(line)

			if procCount >= len(ret) {
				ret = append(ret, asmProc{name: procName})
			}
			proc := &ret[procCount]
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
