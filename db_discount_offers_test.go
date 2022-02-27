package main

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDiscountID(t *testing.T) {
	last := uint(len(discountOffers))
	exceeded := last + 1
	exceededErr := eDiscountIDOutOfRange{exceeded}

	type testcase struct {
		s   string
		id  uint
		err error
	}
	verify := func(tc *testcase, id DiscountID, err error, testname string) {
		assert.Equal(t, DiscountID(tc.id), id, testname)
		if errors.Is(tc.err, assert.AnError) {
			assert.Error(t, err)
		} else {
			assert.ErrorIs(t, err, tc.err)
		}
	}

	testcases := []testcase{
		{"0", 0, nil},
		{strconv.FormatUint(uint64(last), 32), last, nil},
		{strconv.FormatUint(uint64(exceeded), 32), 0, exceededErr},
		{"a string", 0, assert.AnError},
		{"-1", 0, assert.AnError},
	}

	for _, tc := range testcases {
		id, err := NewDiscountID(tc.s)
		verify(&tc, id, err, "regular")
		verify(&tc, id, id.UnmarshalCSV(tc.s), "CSV")
		verify(&tc, id, id.UnmarshalJSON([]byte(tc.s)), "JSON")
	}
}
