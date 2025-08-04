package frankenphp

/*
#include "types.h"
*/
import "C"
import "unsafe"

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

// PHPKeyType represents the type of PHP hashmap key
type PHPKeyType int

const (
	PHPIntKey PHPKeyType = iota
	PHPStringKey
)

type PHPKey struct {
	Type PHPKeyType
	Str  string
	Int  int64
}

// Array represents a PHP array with ordered key-value pairs
type Array struct {
	keys   []PHPKey
	values []interface{}
}

// SetInt sets a value with an integer key
func (arr *Array) SetInt(key int64, value interface{}) {
	arr.keys = append(arr.keys, PHPKey{Type: PHPIntKey, Int: key})
	arr.values = append(arr.values, value)
}

// SetString sets a value with a string key
func (arr *Array) SetString(key string, value interface{}) {
	arr.keys = append(arr.keys, PHPKey{Type: PHPStringKey, Str: key})
	arr.values = append(arr.values, value)
}

// Append adds a value to the end of the array with the next available integer key
func (arr *Array) Append(value interface{}) {
	nextKey := arr.getNextIntKey()
	arr.SetInt(nextKey, value)
}

// getNextIntKey finds the next available integer key
func (arr *Array) getNextIntKey() int64 {
	maxKey := int64(-1)
	for _, key := range arr.keys {
		if key.Type == PHPIntKey && key.Int > maxKey {
			maxKey = key.Int
		}
	}

	return maxKey + 1
}

// Len returns the number of elements in the array
func (arr *Array) Len() uint32 {
	return uint32(len(arr.keys))
}

// At returns the key and value at the given index
func (arr *Array) At(index uint32) (PHPKey, interface{}) {
	if index >= uint32(len(arr.keys)) {
		return PHPKey{}, nil
	}
	return arr.keys[index], arr.values[index]
}

// EXPERIMENTAL: GoArray converts a zend_array to a Go Array
func GoArray(arr unsafe.Pointer) *Array {
	result := &Array{
		keys:   make([]PHPKey, 0),
		values: make([]interface{}, 0),
	}

	if arr == nil {
		return result
	}

	zval := (*C.zval)(arr)
	hashTable := (*C.HashTable)(castZval(zval, C.IS_ARRAY))

	if hashTable == nil {
		return result
	}

	used := hashTable.nNumUsed
	if htIsPacked(hashTable) {
		for i := C.uint32_t(0); i < used; i++ {
			v := C.get_ht_packed_data(hashTable, i)
			if v != nil && C.zval_get_type(v) != C.IS_UNDEF {
				value := convertZvalToGo(v)
				result.SetInt(int64(i), value)
			}
		}

		return result
	}

	for i := C.uint32_t(0); i < used; i++ {
		bucket := C.get_ht_bucket_data(hashTable, i)
		if bucket == nil || C.zval_get_type(&bucket.val) == C.IS_UNDEF {
			continue
		}

		v := convertZvalToGo(&bucket.val)

		if bucket.key != nil {
			keyStr := GoString(unsafe.Pointer(bucket.key))
			result.SetString(keyStr, v)

			continue
		}

		result.SetInt(int64(bucket.h), v)
	}

	return result
}

// PHPArray converts a Go Array to a PHP zend_array.
func PHPArray(arr *Array) unsafe.Pointer {
	if arr == nil || arr.Len() == 0 {
		return unsafe.Pointer(createNewArray(0))
	}

	isList := true
	for i, k := range arr.keys {
		if k.Type != PHPIntKey || k.Int != int64(i) {
			isList = false
			break
		}
	}

	var zendArray *C.HashTable
	if isList {
		zendArray = createNewArray(arr.Len())
		for _, v := range arr.values {
			zval := convertGoToZval(v)
			C.zend_hash_next_index_insert(zendArray, zval)
		}

		return unsafe.Pointer(zendArray)
	}

	zendArray = createNewArray(arr.Len())
	for i, k := range arr.keys {
		zval := convertGoToZval(arr.values[i])

		if k.Type == PHPStringKey {
			keyStr := k.Str
			keyData := (*C.char)(unsafe.Pointer(unsafe.StringData(keyStr)))
			C.zend_hash_str_add(zendArray, keyData, C.size_t(len(keyStr)), zval)

			continue
		}

		C.zend_hash_index_update(zendArray, C.zend_ulong(k.Int), zval)
	}

	return unsafe.Pointer(zendArray)
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
	case *Array:
		arr := (*C.zend_array)(PHPArray(v))
		C.__zval_arr__(&zval, arr)
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
