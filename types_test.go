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

func TestPHPAssociativeArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalArray := AssociativeArray{Map: map[string]interface{}{
			"foo1": "bar1",
			"foo2": "bar2",
		}}

		convertedArray, err := GoAssociativeArray(PHPAssociativeArray(originalArray), false)

		assert.NoError(t, err)
		assert.Equal(t, originalArray, convertedArray, "associative array should be equal after conversion")
	})
}

func TestOrderedPHPAssociativeArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalArray := AssociativeArray{
			Map: map[string]interface{}{
				"foo1": "bar1",
				"foo2": "bar2",
			},
			Order: []string{"foo2", "foo1"},
		}

		convertedArray, err := GoAssociativeArray(PHPAssociativeArray(originalArray), true)

		assert.NoError(t, err)
		assert.Equal(t, originalArray, convertedArray, "associative array should be equal after conversion")
	})
}

func TestPHPPackedArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalArray := PackedArray{"bar1", "bar2"}

		convertedArray, err := GoPackedArray(PHPPackedArray(originalArray))

		assert.NoError(t, err)
		assert.Equal(t, originalArray, convertedArray, "packed array should be equal after conversion")
	})
}

func TestPHPPackedArrayToAssociative(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalArray := PackedArray{"bar1", "bar2"}
		expectedArray := AssociativeArray{Map: map[string]interface{}{
			"0": "bar1",
			"1": "bar2",
		}}

		convertedArray, err := GoAssociativeArray(PHPPackedArray(originalArray), false)

		assert.NoError(t, err)
		assert.Equal(t, expectedArray, convertedArray, "convert a packed to an associative array")
	})
}

func TestPHPAssociativeArrayToPacked(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalArray := AssociativeArray{
			Map: map[string]interface{}{
				"foo1": "bar1",
				"foo2": "bar2",
			},
			Order: []string{"foo1", "foo2"},
		}
		expectedArray := PackedArray{"bar1", "bar2"}

		convertedArray, err := GoPackedArray(PHPAssociativeArray(originalArray))

		assert.NoError(t, err)
		assert.Equal(t, expectedArray, convertedArray, "convert an associative to a packed array")
	})
}

func TestNestedMixedArray(t *testing.T) {
	testOnDummyPHPThread(t, func() {
		originalArray := AssociativeArray{
			Map: map[string]interface{}{
				"foo":         "bar",
				"int":         int64(123),
				"float":       float64(1.2),
				"true":        true,
				"false":       false,
				"nil":         nil,
				"packedArray": PackedArray{"bar1", "bar2"},
				"associativeArray": AssociativeArray{
					Map:   map[string]interface{}{"foo1": "bar1", "foo2": "bar2"},
					Order: []string{"foo2", "foo1"},
				},
			},
		}

		convertedArray, err := GoAssociativeArray(PHPAssociativeArray(originalArray), false)

		assert.NoError(t, err)
		assert.Equal(t, originalArray, convertedArray, "nested mixed array should be equal after conversion")
	})
}
