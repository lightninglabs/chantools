package btc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var testCases = []struct {
	descriptor  string
	expectedSum string
}{{
	descriptor:  "addr(mkmZxiEcEd8ZqjQWVZuC6so5dFMKEFpN2j)",
	expectedSum: "#02wpgw69",
}, {
	descriptor:  "tr(cRhCT5vC5NdnSrQ2Jrah6NPCcth41uT8DWFmA6uD8R4x2ufucnYX)",
	expectedSum: "#gwfmkgga",
}}

func TestDescriptorSum(t *testing.T) {
	for _, tc := range testCases {
		sum := DescriptorSumCreate(tc.descriptor)
		require.Equal(t, tc.descriptor+tc.expectedSum, sum)

		DescriptorSumCheck(sum, true)
	}
}
