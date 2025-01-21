package phpheaders

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAllHeadersAreCorrect(t *testing.T) {
	for header, phpHeader := range commonRequestHeaders {
		expectedPHPHeader := GetUnCommonHeader(header)
		// trim the null byte from the expectedPHPHeader
		expectedPHPHeader = expectedPHPHeader[:len(expectedPHPHeader)-1]
		assert.Equal(t, phpHeader, expectedPHPHeader, "header is not well formed: "+phpHeader)
	}
}
