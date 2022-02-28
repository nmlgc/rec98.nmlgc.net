package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveDuplicates(t *testing.T) {
	in := []int{1, 2, 2, 3, 4, 5, 5, 5, 6, 7, 8, 9, 9, 9, 10}
	exp := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	RemoveDuplicates(&in)

	assert.Equal(t, in, exp)
}
