package phpheaders

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllHeadersAreCorrect(t *testing.T) {
	for header, phpHeader := range commonRequestHeaders {
		expectedPHPHeader := GetUnCommonHeader(header)
		hardCodedHeader := phpHeader + "\x00"
		assert.Equal(t, hardCodedHeader, expectedPHPHeader, "header is not well formed: "+phpHeader)
	}
}
