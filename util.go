package main

import (
	"crypto/sha512"
	"os"
)

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

// CryptHash represents a hash created by a current cryptographically secure
// algorithm.
type CryptHash [sha512.Size]byte

// CryptHashOfFile hashes the file with the given name using a current
// cryptographically secure algorithm.
func CryptHashOfFile(fn string) CryptHash {
	f, err := os.ReadFile(fn)
	FatalIf(err)
	return sha512.Sum512(f)
}
