package etcdext

/*
#include "etcd.h"
*/
import "C"
import (
    "context"
    "time"
    "unsafe"

    clientv3 "go.etcd.io/etcd/client/v3"
    "github.com/dunglas/frankenphp"
)

//export_php:function etcd_get(string $key): string
func etcd_get(keyStr *C.zend_string) unsafe.Pointer {
    key := frankenphp.GoString(unsafe.Pointer(keyStr))

    cli, err := clientv3.New(clientv3.Config{
        Endpoints:   []string{"localhost:2379"},
        DialTimeout: 2 * time.Second,
    })
    if err != nil {
        return frankenphp.PhpString("", false)
    }
    defer cli.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    resp, err := cli.Get(ctx, key)
    cancel()
    if err != nil || len(resp.Kvs) == 0 {
        return frankenphp.PhpString("", false)
    }
    return frankenphp.PhpString(string(resp.Kvs[0].Value), false)
}

//export_php:function etcd_delete(string $key): bool
func etcd_delete(keyStr *C.zend_string) C.bool {
    key := frankenphp.GoString(unsafe.Pointer(keyStr))

    cli, err := clientv3.New(clientv3.Config{
        Endpoints:   []string{"localhost:2379"},
        DialTimeout: 2 * time.Second,
    })
    if err != nil {
        return C.FALSE
    }
    defer cli.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    _, err = cli.Delete(ctx, key)
    cancel()
    if err != nil {
        return C.FALSE
    }
    return C.TRUE
}
