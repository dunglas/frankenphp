package frankenphp

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

// execute the function on a PHP thread directly
// this is necessary if tests make use of PHP's internal allocation
func testOnDummyPHPThread(t *testing.T, test func()) {
	t.Helper()
	logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := initPHPThreads(1, 1, nil) // boot 1 thread
	assert.NoError(t, err)
	handler := convertToTaskThread(phpThreads[0])

	task := newTask(test)
	handler.execute(task)
	task.waitForCompletion()

	drainPHPThreads()
}

func TestGoString(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalString := "Hello, World!"

		convertedString := GoString(PHPString(originalString, false))

		assert.Equal(t, originalString, convertedString, "string -> zend_string -> string should yield an equal string")
	})
}

func TestPHPArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalArray := NewAssociativeArray(
			KeyValuePair{"foo1", "bar1"},
			KeyValuePair{"foo2", "bar2"},
		)
		originalArray.Set("foo3", "bar3")

		convertedArray := GoArray(PHPArray(originalArray))

		assert.Equal(t, originalArray, convertedArray, "associative array should be equal after conversion")
		assert.Len(t, convertedArray.Entries(), 3)
		assert.Len(t, convertedArray.Values(), 3)
		foo1, exists := convertedArray.Get("foo1")
		assert.True(t, exists, "key 'foo1' should exist in the converted array")
		assert.Equal(t, "bar1", foo1, "value for key 'foo1' should be 'bar1'")
		_, exists = convertedArray.Get("foo4")
		assert.False(t, exists, "key 'foo4' should exist in the converted array")
	})
}

func TestPHPPackedArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalArray := NewPackedArray(
			"bar1",
			"bar2",
		)
		err := originalArray.Append("bar3")

		goArray := GoArray(PHPArray(originalArray))

		assert.NoError(t, err)
		assert.Equal(t, originalArray, goArray, "packed array should be equal after conversion")
		bar1, err := goArray.GetAtIndex(0)
		assert.NoError(t, err, "GetAtIndex should not return an error for packed arrays")
		assert.Equal(t, "bar1", bar1, "first element should be 'bar1'")
		bar3, err := goArray.GetAtIndex(2)
		assert.NoError(t, err, "GetAtIndex should not return an error for packed arrays")
		assert.Equal(t, "bar3", bar3, "third element should be 'bar3'")
		_, err = goArray.GetAtIndex(3)
		assert.Error(t, err, "GetAtIndex should return an error for out-of-bounds index on packed arrays")
	})
}

func TestNestedMixedArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalArray := NewAssociativeArray(
			KeyValuePair{"array", NewAssociativeArray(
				KeyValuePair{"foo", "bar"},
			)},
			KeyValuePair{"packedArray", NewPackedArray(
				float64(1.1),
				int64(2),
				"packed",
			)},
			KeyValuePair{"integer", int64(1)},
			KeyValuePair{"float", float64(1.2)},
			KeyValuePair{"string", "bar"},
			KeyValuePair{"null", nil},
			KeyValuePair{"false", false},
			KeyValuePair{"true", true},
		)

		zendArray := PHPArray(originalArray)
		goArray := GoArray(zendArray)

		assert.Equal(t, originalArray, goArray, "nested mixed array should be equal after conversion")
	})
}

func TestArrayShouldBeCorrectlyPacked(t *testing.T) {
	originalArray := NewPackedArray("bar1", "bar2")

	assert.True(t, originalArray.IsPacked(), "Array should be packed")

	err := originalArray.Append("bar3")

	assert.NoError(t, err)
	assert.Equal(t, 3, len(originalArray.packedValues), "Packed array should have 3 elements after appending")
	assert.True(t, originalArray.IsPacked(), "Array should be packed after Append")

	err = originalArray.SetAtIndex(1, "newBar2")

	assert.NoError(t, err, "SetAtIndex should not return an error for packed arrays")
	assert.Equal(t, "newBar2", originalArray.Values()[1], "Second element should be updated to 'newBar2'")
	assert.True(t, originalArray.IsPacked(), "Array should be packed after SetAtIndex")

	originalArray.Set("hello", "world")
	assert.False(t, originalArray.IsPacked(), "Array should not be packed anymore after setting a string offset")

	assert.Equal(t, NewAssociativeArray(
		KeyValuePair{"0", "bar1"},
		KeyValuePair{"1", "newBar2"},
		KeyValuePair{"2", "bar3"},
		KeyValuePair{"hello", "world"},
	), originalArray, "Array should be associated after mixed operations")
}
