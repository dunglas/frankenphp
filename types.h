#ifndef TYPES_H
#define TYPES_H

#include <zend.h>
#include <zend_API.h>
#include <zend_alloc.h>
#include <zend_hash.h>
#include <zend_types.h>

zval *get_ht_packed_data(HashTable *, uint32_t index);
Bucket *get_ht_bucket_data(HashTable *, uint32_t index);

void *__emalloc__(size_t size);
void __zend_hash_init__(HashTable *ht, uint32_t nSize, dtor_func_t pDestructor,
                        bool persistent);

#endif
