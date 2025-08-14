package frankenphp

/*
#include "types.h"
*/
import "C"
import (
	"errors"
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

// AssociativeArray represents a PHP array with ordered key-value pairs
type AssociativeArray struct {
	Map   map[string]interface{}
	Order []string
}

// PackedArray represents a 'packed' PHP array as a Go slice
type PackedArray = []interface{}

// EXPERIMENTAL: GoAssociativeArray converts a zend_array to a Go AssociativeArray
func GoAssociativeArray(arr unsafe.Pointer, ordered bool) (AssociativeArray, error) {
	if arr == nil {
		return AssociativeArray{}, errors.New("GoAssociativeArray received a nil pointer")
	}

	zval := (*C.zval)(arr)
	hashTable := (*C.HashTable)(castZval(zval, C.IS_ARRAY))

	if hashTable == nil {
		return AssociativeArray{}, errors.New("GoAssociativeArray received a zval that was not a HashTable")
	}

	nNumUsed := hashTable.nNumUsed
	result := AssociativeArray{Map: make(map[string]interface{})}
	if ordered {
		result.Order = make([]string, 0, nNumUsed)
	}

	if htIsPacked(hashTable) {
		// if the HashTable is packed, convert all integer keys to strings
		// this is probably a bug by the dev using this function
		// still, we'll (inefficiently) convert to an associative array
		for i := C.uint32_t(0); i < nNumUsed; i++ {
			v := C.get_ht_packed_data(hashTable, i)
			if v != nil && C.zval_get_type(v) != C.IS_UNDEF {
				strIndex := strconv.Itoa(int(i))
				result.Map[strIndex] = convertZvalToGo(v)
				if ordered {
					result.Order = append(result.Order, strIndex)
				}
			}
		}

		return result, nil
	}

	for i := C.uint32_t(0); i < nNumUsed; i++ {
		bucket := C.get_ht_bucket_data(hashTable, i)
		if bucket == nil || C.zval_get_type(&bucket.val) == C.IS_UNDEF {
			continue
		}

		v := convertZvalToGo(&bucket.val)

		if bucket.key != nil {
			keyStr := GoString(unsafe.Pointer(bucket.key))
			result.Map[keyStr] = v
			if ordered {
				result.Order = append(result.Order, keyStr)
			}

			continue
		}

		// as fallback convert the bucket index to a string key
		strIndex := strconv.Itoa(int(bucket.h))
		result.Map[strIndex] = v
		if ordered {
			result.Order = append(result.Order, strIndex)
		}
	}

	return result, nil
}

// EXPERIMENTAL: GoPackedArray converts a zend_array to a Go slice
func GoPackedArray(arr unsafe.Pointer) (PackedArray, error) {
	if arr == nil {
		return PackedArray{}, errors.New("GoPackedArray received a nil pointer")
	}

	zval := (*C.zval)(arr)
	hashTable := (*C.HashTable)(castZval(zval, C.IS_ARRAY))

	if hashTable == nil {
		return PackedArray{}, errors.New("GoPackedArray received zval that wasn'T a HashTable")
	}

	nNumUsed := hashTable.nNumUsed
	result := make(PackedArray, 0, nNumUsed)

	if htIsPacked(hashTable) {
		for i := C.uint32_t(0); i < nNumUsed; i++ {
			v := C.get_ht_packed_data(hashTable, i)
			if v != nil && C.zval_get_type(v) != C.IS_UNDEF {
				result = append(result, convertZvalToGo(v))
			}
		}

		return result, nil
	}

	// fallback if ht isn't packed - equivalent to array_values()
	for i := C.uint32_t(0); i < nNumUsed; i++ {
		bucket := C.get_ht_bucket_data(hashTable, i)
		if bucket != nil && C.zval_get_type(&bucket.val) != C.IS_UNDEF {
			result = append(result, convertZvalToGo(&bucket.val))
		}
	}

	return result, nil
}

// PHPAssociativeArray converts a Go AssociativeArray to a PHP zend_array.
func PHPAssociativeArray(arr AssociativeArray) unsafe.Pointer {
	var zendArray *C.HashTable

	if len(arr.Order) != 0 {
		zendArray = createNewArray((uint32)(len(arr.Order)))
		for _, key := range arr.Order {
			val := arr.Map[key]
			zval := convertGoToZval(val)
			C.zend_hash_str_update(zendArray, toUnsafeChar(key), C.size_t(len(key)), zval)
		}
	} else {
		zendArray = createNewArray((uint32)(len(arr.Map)))
		for key, val := range arr.Map {
			zval := convertGoToZval(val)
			C.zend_hash_str_update(zendArray, toUnsafeChar(key), C.size_t(len(key)), zval)
		}
	}

	var zval C.zval
	C.__zval_arr__(&zval, zendArray)

	return unsafe.Pointer(&zval)
}

// PHPPackedArray converts a Go PHPPackedArray to a PHP zend_array.
func PHPPackedArray(arr PackedArray) unsafe.Pointer {
	zendArray := createNewArray((uint32)(len(arr)))
	for _, val := range arr {
		zval := convertGoToZval(val)
		C.zend_hash_next_index_insert(zendArray, zval)
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
		hashTable := (*C.HashTable)(castZval(zval, C.IS_ARRAY))
		if hashTable != nil && htIsPacked(hashTable) {
			packedArray, _ := GoPackedArray(unsafe.Pointer(zval))

			return packedArray
		}
		associativeArray, _ := GoAssociativeArray(unsafe.Pointer(zval), true)

		return associativeArray
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
	case PackedArray:
		return (*C.zval)(PHPPackedArray(v))
	case AssociativeArray:
		return (*C.zval)(PHPAssociativeArray(v))
	case map[string]interface{}:
		return (*C.zval)(PHPAssociativeArray(AssociativeArray{Map: v}))
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
