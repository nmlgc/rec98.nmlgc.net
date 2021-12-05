package main

import (
	"crypto/sha512"
	"io/ioutil"
	"reflect"
)

// RemoveDuplicates was adapted from
// https://www.rosettacode.org/wiki/Remove_duplicate_elements#Any_type_using_reflection
// to modify the slice in-place.
func RemoveDuplicates(x interface{}) {
	p := reflect.ValueOf(x)
	if !p.IsValid() {
		panic("RemoveDuplicates: invalid argument")
	}
	pk := p.Kind()
	if pk != reflect.Ptr {
		panic("RemoveDuplicates: argument must be pointer to array or a slice")
	}
	v := p.Elem()
	vk := v.Kind()
	if vk != reflect.Array && vk != reflect.Slice {
		panic("RemoveDuplicates: argument must be pointer to array or a slice")
	}
	elemType := v.Type().Elem()
	intType := reflect.TypeOf(int(0))
	mapType := reflect.MapOf(elemType, intType)
	m := reflect.MakeMapWithSize(mapType, v.Len())
	idx := 0
	for j := 0; j < v.Len(); j++ {
		x := v.Index(j)
		if m.MapIndex(x).IsValid() {
			continue
		}
		m.SetMapIndex(x, reflect.ValueOf(idx))
		if m.MapIndex(x).IsValid() {
			idx++
		}
	}
	for _, key := range m.MapKeys() {
		idx := m.MapIndex(key)
		v.Index(int(idx.Int())).Set(key)
	}
	v.SetLen(m.Len())
}

// CryptHash represents a hash created by a current cryptographically secure
// algorithm.
type CryptHash [sha512.Size]byte

// CryptHashOfFile hashes the file with the given name using a current
// cryptographically secure algorithm.
func CryptHashOfFile(fn string) CryptHash {
	f, err := ioutil.ReadFile(fn)
	FatalIf(err)
	return sha512.Sum512(f)
}
