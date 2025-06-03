package frankenphp

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

func TestGoString(t *testing.T) {
	t.Run("nil_pointer", func(t *testing.T) {
		result := GoString(nil)
		assert.Equal(t, "", result)
	})

	t.Run("zero_pointer", func(t *testing.T) {
		result := GoString(unsafe.Pointer(uintptr(0)))
		assert.Equal(t, "", result)
	})
}

func TestGoStringCopy(t *testing.T) {
	t.Run("nil_pointer", func(t *testing.T) {
		result := GoStringCopy(nil)
		assert.Equal(t, "", result)
	})

	t.Run("zero_pointer", func(t *testing.T) {
		result := GoStringCopy(unsafe.Pointer(uintptr(0)))
		assert.Equal(t, "", result)
	})
}

func TestPHPString(t *testing.T) {
	t.Run("empty_string_non_persistent", func(t *testing.T) {
		result := PHPString("", false)
		assert.Nil(t, result)
	})

	t.Run("empty_string_persistent", func(t *testing.T) {
		result := PHPString("", true)
		assert.Nil(t, result)
	})

	t.Run("non_empty_string_non_persistent", func(t *testing.T) {
		result := PHPString("hello", false)
		assert.NotNil(t, result)
	})

	t.Run("non_empty_string_persistent", func(t *testing.T) {
		result := PHPString("hello", true)
		assert.NotNil(t, result)
	})
}
