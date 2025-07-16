#include "types.h"

zval *get_ht_packed_data(HashTable *ht, uint32_t index) {
  if (ht->u.flags & HASH_FLAG_PACKED) {
    return &ht->arPacked[index];
  }
  return NULL;
}

Bucket *get_ht_bucket_data(HashTable *ht, uint32_t index) {
  if (!(ht->u.flags & HASH_FLAG_PACKED)) {
    return &ht->arData[index];
  }
  return NULL;
}

void *__emalloc__(size_t size) { return emalloc(size); }

void __zend_hash_init__(HashTable *ht, uint32_t nSize, dtor_func_t pDestructor,
                        bool persistent) {
  zend_hash_init(ht, nSize, null, pDestructor, persistent);
}
