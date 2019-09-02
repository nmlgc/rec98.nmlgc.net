package main

import (
	"reflect"
	"testing"
)

func TestRemoveDuplicates(t *testing.T) {
	in := []int{1, 2, 2, 3, 4, 5, 5, 5, 6, 7, 8, 9, 9, 9, 10}
	exp := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	RemoveDuplicates(&in)
	if !reflect.DeepEqual(in, exp) {
		t.Errorf("removeDuplicates:\n"+
			"wanted %v,\n"+
			"   got %v\n", exp, in)
	}
}
