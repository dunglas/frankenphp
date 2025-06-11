package testext

//#include "extension.h"
import "C"
import (
	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http/httptest"
	"testing"
	"unsafe"
)

func testRegisterExtension(t *testing.T) {
	frankenphp.RegisterExtension(unsafe.Pointer(&C.module1_entry))
	frankenphp.RegisterExtension(unsafe.Pointer(&C.module2_entry))

	err := frankenphp.Init()
	require.Nil(t, err)
	defer frankenphp.Shutdown()

	req := httptest.NewRequest("GET", "http://example.com/index.php", nil)
	w := httptest.NewRecorder()

	req, err = frankenphp.NewRequestWithContext(req, frankenphp.WithRequestDocumentRoot("./testdata", false))
	assert.NoError(t, err)

	err = frankenphp.ServeHTTP(w, req)
	assert.NoError(t, err)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "ext1")
	assert.Contains(t, string(body), "ext2")
}
