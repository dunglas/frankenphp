package headers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllHardCodedHeadersAreWellFormed(t *testing.T) {
	for header, phpHeader := range commonRequestHeaders {
		expectedPHPHeader := GetUnCommonHeader(header)
		hardCodedHeader := phpHeader + "\x00"
		assert.Equal(t, hardCodedHeader, expectedPHPHeader, "header %s is not well formed", header)
	}
}
