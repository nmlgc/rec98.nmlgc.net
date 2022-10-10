package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Well, since golang.org/x/exp/constraints is not stable…
func TestMinMax(t *testing.T) {
	a := 1
	b := 2

	assert.Equal(t, Min(int8(a), int8(b)), int8(a))
	assert.Equal(t, Min(int16(a), int16(b)), int16(a))
	assert.Equal(t, Min(int32(a), int32(b)), int32(a))
	assert.Equal(t, Min(int64(a), int64(b)), int64(a))
	assert.Equal(t, Max(int8(a), int8(b)), int8(b))
	assert.Equal(t, Max(int16(a), int16(b)), int16(b))
	assert.Equal(t, Max(int32(a), int32(b)), int32(b))
	assert.Equal(t, Max(int64(a), int64(b)), int64(b))
}

func TestRightPad(t *testing.T) {
	assert.Equal(t, RightPad("foo", 5), "foo  ")
	assert.Equal(t, RightPad("foo", 0), "foo")
	assert.Equal(t, RightPad("foo", -1), "foo")
}

func TestRemoveDuplicates(t *testing.T) {
	in := []int{1, 2, 2, 3, 4, 5, 5, 5, 6, 7, 8, 9, 9, 9, 10}
	exp := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	RemoveDuplicates(&in)

	assert.Equal(t, in, exp)
}
