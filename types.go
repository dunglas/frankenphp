package frankenphp

/*
#include "types.h"
*/
import "C"
import (
	"fmt"
	"strconv"
	"unsafe"
)

// EXPERIMENTAL: GoString copies a zend_string to a Go string.
func GoString(s unsafe.Pointer) string {
	if s == nil {
		return ""
	}

	zendStr := (*C.zend_string)(s)

	return C.GoStringN((*C.char)(unsafe.Pointer(&zendStr.val)), C.int(zendStr.len))
}

// EXPERIMENTAL: PHPString converts a Go string to a zend_string with copy. The string can be
// non-persistent (automatically freed after the request by the ZMM) or persistent. If you choose
// the second mode, it is your repsonsability to free the allocated memory.
func PHPString(s string, persistent bool) unsafe.Pointer {
	if s == "" {
		return nil
	}

	zendStr := C.zend_string_init(
		(*C.char)(unsafe.Pointer(unsafe.StringData(s))),
		C.size_t(len(s)),
		C._Bool(persistent),
	)

	return unsafe.Pointer(zendStr)
}

type KeyValuePair struct {
	key   string
	value interface{}
}

// Array represents a PHP array with ordered key-value pairs
type Array struct {
	isPacked     bool
	pairs        map[string]KeyValuePair
	entries      []KeyValuePair
	packedValues []interface{}
}

// EXPERIMENTAL: NewAssociativeArray creates a new associative array
func NewAssociativeArray(entries ...KeyValuePair) Array {
	arr := Array{
		pairs:   make(map[string]KeyValuePair),
		entries: entries,
	}
	for _, pair := range entries {
		arr.pairs[pair.key] = pair
	}

	return arr
}

// EXPERIMENTAL: NewPackedArray creates a new packed array
func NewPackedArray(values ...interface{}) Array {
	arr := Array{
		isPacked:     true,
		packedValues: values,
	}

	return arr
}

// Get value (for associative arrays)
func (arr *Array) Get(key string) (interface{}, bool) {
	if pair, exists := arr.pairs[key]; exists {
		return pair.value, true
	}
	return nil, false
}

// Set value (for associative arrays)
func (arr *Array) Set(key string, value interface{}) {
	if pair, exists := arr.pairs[key]; exists {
		pair.value = value
		return
	}

	// setting a key in a packed array will make it an associative array
	if arr.isPacked {
		arr.isPacked = false
		for i, v := range arr.packedValues {
			arr.Set(strconv.Itoa(i), v)
		}
		arr.packedValues = nil
	}

	newPair := KeyValuePair{key: key, value: value}
	arr.entries = append(arr.entries, newPair)
	if arr.pairs == nil {
		arr.pairs = make(map[string]KeyValuePair)
	}
	arr.pairs[key] = newPair
}

// Set value at index (for packed arrays)
func (arr *Array) SetAtIndex(index int, value interface{}) error {
	if !arr.isPacked {
		return fmt.Errorf("SetAtIndex is only supported for packed arrays, use Set instead")
	}

	if index < 0 || index >= len(arr.packedValues) {
		return fmt.Errorf("Index %d out of bounds for packed array with length %d", index, len(arr.packedValues))
	}
	arr.packedValues[index] = value

	return nil
}

// Get value at index (for packed arrays)
func (arr *Array) GetAtIndex(index int) (interface{}, error) {
	if !arr.isPacked {
		return nil, fmt.Errorf("GetAtIndex is only supported for packed arrays, use Get instead")
	}

	if index < 0 || index >= len(arr.packedValues) {
		return nil, fmt.Errorf("Index %d out of bounds for packed array with length %d", index, len(arr.packedValues))
	}

	return arr.packedValues[index], nil
}

// Append to the array (only for packed arrays)
func (arr *Array) Append(value interface{}) error {
	if !arr.isPacked {
		return fmt.Errorf("Append is not supported for associative arrays, use Set instead")
	}

	arr.packedValues = append(arr.packedValues, value)

	return nil
}

func (arr *Array) IsPacked() bool {
	return arr.isPacked
}

// Entries returns all ordered key-value pairs, best performance for associative arrays
func (arr *Array) Entries() []KeyValuePair {
	if !arr.isPacked {
		return arr.entries
	}
	entries := make([]KeyValuePair, 0, len(arr.packedValues))
	for i, value := range arr.packedValues {
		entries = append(entries, KeyValuePair{key: strconv.Itoa(i), value: value})
	}

	return entries
}

// Values returns all values, best performance for packed arrays
func (arr *Array) Values() []interface{} {
	if arr.isPacked {
		return arr.packedValues
	}
	values := make([]interface{}, 0, len(arr.entries))
	for _, pair := range arr.entries {
		values = append(values, pair.value)
	}

	return values
}

// EXPERIMENTAL: GoArray converts a zend_array to a Go Array
func GoArray(arr unsafe.Pointer) Array {
	if arr == nil {
		fmt.Println("GoArray received a nil pointer")
		return Array{}
	}

	zval := (*C.zval)(arr)
	hashTable := (*C.HashTable)(castZval(zval, C.IS_ARRAY))

	if hashTable == nil {
		fmt.Println("GoArray received a nil pointer")
		return Array{}
	}

	nNumUsed := hashTable.nNumUsed
	result := Array{}

	if htIsPacked(hashTable) {
		result.isPacked = true
		result.packedValues = make([]interface{}, 0, nNumUsed)
		for i := C.uint32_t(0); i < nNumUsed; i++ {
			v := C.get_ht_packed_data(hashTable, i)
			if v != nil && C.zval_get_type(v) != C.IS_UNDEF {
				result.packedValues = append(result.packedValues, convertZvalToGo(v))
			}
		}

		return result
	}

	for i := C.uint32_t(0); i < nNumUsed; i++ {
		bucket := C.get_ht_bucket_data(hashTable, i)
		if bucket == nil || C.zval_get_type(&bucket.val) == C.IS_UNDEF {
			continue
		}

		v := convertZvalToGo(&bucket.val)

		if bucket.key != nil {
			keyStr := GoString(unsafe.Pointer(bucket.key))
			result.Set(keyStr, v)

			continue
		}

		// as fallback convert the bucket index to a string key
		result.Set(strconv.Itoa(int(bucket.h)), v)
	}

	return result
}

// PHPArray converts a Go Array to a PHP zend_array.
func PHPArray(arr Array) unsafe.Pointer {
	zendArray := createNewArray((uint32)(len(arr.entries)))

	if arr.isPacked {
		for _, v := range arr.packedValues {
			zval := convertGoToZval(v)
			C.zend_hash_next_index_insert(zendArray, zval)
		}
	} else {
		for _, pair := range arr.entries {
			zval := convertGoToZval(pair.value)

			keyStr := PHPString(pair.key, false)
			C.zend_hash_update(zendArray, (*C.zend_string)(keyStr), zval)
		}
	}

	var zval C.zval
	C.__zval_arr__(&zval, zendArray)

	return unsafe.Pointer(&zval)
}

// convertZvalToGo converts a PHP zval to a Go interface{}
func convertZvalToGo(zval *C.zval) interface{} {
	t := C.zval_get_type(zval)
	switch t {
	case C.IS_NULL:
		return nil
	case C.IS_FALSE:
		return false
	case C.IS_TRUE:
		return true
	case C.IS_LONG:
		longPtr := (*C.zend_long)(castZval(zval, C.IS_LONG))
		if longPtr != nil {
			return int64(*longPtr)
		}
		return int64(0)
	case C.IS_DOUBLE:
		doublePtr := (*C.double)(castZval(zval, C.IS_DOUBLE))
		if doublePtr != nil {
			return float64(*doublePtr)
		}
		return float64(0)
	case C.IS_STRING:
		str := (*C.zend_string)(castZval(zval, C.IS_STRING))
		if str == nil {
			return ""
		}

		return GoString(unsafe.Pointer(str))
	case C.IS_ARRAY:
		return GoArray(unsafe.Pointer(zval))
	default:
		return nil
	}
}

// convertGoToZval converts a Go interface{} to a PHP zval
func convertGoToZval(value interface{}) *C.zval {
	var zval C.zval

	switch v := value.(type) {
	case nil:
		C.__zval_null__(&zval)
	case bool:
		C.__zval_bool__(&zval, C._Bool(v))
	case int:
		C.__zval_long__(&zval, C.zend_long(v))
	case int64:
		C.__zval_long__(&zval, C.zend_long(v))
	case float64:
		C.__zval_double__(&zval, C.double(v))
	case string:
		str := (*C.zend_string)(PHPString(v, false))
		C.__zval_string__(&zval, str)
	case Array:
		return (*C.zval)(PHPArray(v))
	default:
		C.__zval_null__(&zval)
	}

	return &zval
}

// createNewArray creates a new zend_array with the specified size.
func createNewArray(size uint32) *C.HashTable {
	arr := C.__zend_new_array__(C.uint32_t(size))
	return (*C.HashTable)(unsafe.Pointer(arr))
}

// htIsPacked checks if a HashTable is a list (packed) or hashmap (not packed).
func htIsPacked(ht *C.HashTable) bool {
	flags := *(*C.uint32_t)(unsafe.Pointer(&ht.u[0]))

	return (flags & C.HASH_FLAG_PACKED) != 0
}

// castZval casts a zval to the expected type and returns a pointer to the value
func castZval(zval *C.zval, expectedType C.uint8_t) unsafe.Pointer {
	if zval == nil || C.zval_get_type(zval) != expectedType {
		return nil
	}

	v := unsafe.Pointer(&zval.value[0])

	switch expectedType {
	case C.IS_LONG:
		return v
	case C.IS_DOUBLE:
		return v
	case C.IS_STRING:
		return unsafe.Pointer(*(**C.zend_string)(v))
	case C.IS_ARRAY:
		return unsafe.Pointer(*(**C.zend_array)(v))
	default:
		return nil
	}
}
