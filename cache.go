// Common functions for serializing and deserializing caches to files.

package main

import (
	"encoding/gob"
	"os"
	"path/filepath"
)

func cacheFN(basename string) string {
	return filepath.Join("cache", basename)
}

// CacheLoad deserializes a cache with the given types from a file with the
// given basename. Returns either valid cache data, or zero-initialized data
// for any I/O error.
func CacheLoad[Payload any](basename string) (ret Payload, err error) {
	f, err := os.Open(cacheFN(basename))
	if err != nil {
		return ret, err
	}
	defer func() { FatalIf(f.Close()) }()

	dec := gob.NewDecoder(f)
	err = dec.Decode(&ret)
	return ret, err
}

// CacheSave serializes the given cache data to a file with the given basename.
func CacheSave(basename string, data any) {
	fn := cacheFN(basename)
	dir, _ := filepath.Split(fn)
	FatalIf(os.MkdirAll(dir, 0600))
	f, err := os.Create(fn)
	FatalIf(err)
	enc := gob.NewEncoder(f)
	FatalIf(enc.Encode(data))
	FatalIf(f.Close())
}
