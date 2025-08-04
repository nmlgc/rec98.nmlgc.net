package main

import (
	"crypto/sha512"
	"encoding/csv"
	"errors"
	"fmt"
	"os"

	"github.com/gocarina/gocsv"
	"golang.org/x/exp/constraints"
)

// Min returns the smaller of two values.
func Min[T constraints.Ordered](a T, b T) T {
	if a < b {
		return a
	}
	return b
}

// Max returns the larger of two values.
func Max[T constraints.Ordered](a T, b T) T {
	if a > b {
		return a
	}
	return b
}

// Concurrent runs the given function concurrently, returning its result on a
// channel.
func Concurrent[T any](f func() T) <-chan T {
	ret := make(chan T)
	go func() {
		ret <- f()
		close(ret)
	}()
	return ret
}

// RemoveDuplicates removes duplicates from the given slice in-place.
func RemoveDuplicates[T comparable](slice *[]T) {
	firstIndexOf := make(map[T]int, len(*slice))
	for _, v := range *slice {
		if _, ok := firstIndexOf[v]; !ok {
			firstIndexOf[v] = len(firstIndexOf)
		}
	}
	for v, i := range firstIndexOf {
		(*slice)[i] = v
	}
	*slice = (*slice)[:len(firstIndexOf)]
}

// RightPad returns s, right-padded with spaces up to the total given padded
// length, or s unchanged if paddedLen is smaller than len(s).
func RightPad(s string, paddedLen int) string {
	return fmt.Sprintf("%s%-*s", s, Max((paddedLen-len(s)), 0), "")
}

// CryptHash represents a hash created by a current cryptographically secure
// algorithm.
type CryptHash [sha512.Size]byte

// CryptHashOfSlice hashes the given byte slice using a current
// cryptographically secure algorithm.
var CryptHashOfSlice = sha512.Sum512

// CryptHashOfFile hashes the file with the given name using a current
// cryptographically secure algorithm.
func CryptHashOfFile(fn string) CryptHash {
	f, err := os.ReadFile(fn)
	FatalIf(err)
	return CryptHashOfSlice(f)
}

// LoadTSV loads a TSV file using the given gocsv unmarshaler.
func LoadTSV(slice any, fn string, unmarshaler func(gocsv.CSVReader, any) error) bool {
	f, err := os.Open(fn)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	FatalIf(err)
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	FatalIf(unmarshaler(reader, slice))
	return true
}
