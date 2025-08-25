package phpheaders

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllCommonHeadersAreCorrect(t *testing.T) {
	fakeRequest := httptest.NewRequest("GET", "http://localhost", nil)

	for header, phpHeader := range CommonRequestHeaders {
		// verify that common and uncommon headers return the same result
		expectedPHPHeader := GetUnCommonHeader(header)
		assert.Equal(t, phpHeader+"\x00", expectedPHPHeader, "header is not well formed: "+phpHeader)

		// net/http will capitalize lowercase headers, verify that headers are capitalized
		fakeRequest.Header.Add(header, "foo")
		_, ok := fakeRequest.Header[header]
		assert.True(t, ok, "header is not correctly capitalized: "+header)
	}
}
