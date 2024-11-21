/* This is a generated file, edit the .stub.php file instead.
 * Stub hash: 05ebde17137c559e891362fba6524fad1e0a2dfe */

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_frankenphp_handle_request, 0, 1,
                                        _IS_BOOL, 0)
ZEND_ARG_TYPE_INFO(0, callback, IS_CALLABLE, 0)
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_headers_send, 0, 0, IS_LONG, 0)
ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, status, IS_LONG, 0, "200")
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_frankenphp_finish_request, 0, 0,
                                        _IS_BOOL, 0)
ZEND_END_ARG_INFO()

#define arginfo_fastcgi_finish_request arginfo_frankenphp_finish_request

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_frankenphp_request_headers, 0,
                                        0, IS_ARRAY, 0)
ZEND_END_ARG_INFO()

#define arginfo_apache_request_headers arginfo_frankenphp_request_headers

#define arginfo_getallheaders arginfo_frankenphp_request_headers

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_MASK_EX(arginfo_frankenphp_response_headers, 0,
                                        0, MAY_BE_ARRAY | MAY_BE_BOOL)
ZEND_END_ARG_INFO()

#define arginfo_apache_response_headers arginfo_frankenphp_response_headers

ZEND_FUNCTION(frankenphp_handle_request);
ZEND_FUNCTION(headers_send);
ZEND_FUNCTION(frankenphp_finish_request);
ZEND_FUNCTION(frankenphp_request_headers);
ZEND_FUNCTION(frankenphp_response_headers);


ZEND_BEGIN_ARG_WITH_RETURN_TYPE_MASK_EX(arginfo_frankenphp_cache_put, 0,
                                        0, MAY_BE_BOOL)
ZEND_ARG_TYPE_INFO(0, key, IS_STRING, 0)
ZEND_ARG_TYPE_INFO(0, value, IS_STRING, 1)
// pass_by_ref, name, type_hint, allow_null, default_value
ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, ttl, IS_LONG, 0, "-1")
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_MASK_EX(arginfo_frankenphp_cache_get, 0,
                                        1, MAY_BE_STRING | MAY_BE_NULL)
ZEND_ARG_TYPE_INFO(0, key, IS_STRING, 0)
ZEND_END_ARG_INFO()
ZEND_BEGIN_ARG_INFO_EX(arginfo_frankenphp_cache_forget, 0, 0, 0)
ZEND_ARG_TYPE_INFO(0, key, IS_STRING, 0)
ZEND_END_ARG_INFO()

#define arginfo_frankenphp_cache_get arginfo_frankenphp_cache_get
#define arginfo_frankenphp_cache_get_forget arginfo_frankenphp_cache_forget
#define arginfo_frankenphp_cache_put arginfo_frankenphp_cache_put
ZEND_FUNCTION(frankenphp_cache_put);
ZEND_FUNCTION(frankenphp_cache_get);
ZEND_FUNCTION(frankenphp_cache_forget);

static const zend_function_entry ext_functions[] = {
    ZEND_FE(frankenphp_handle_request, arginfo_frankenphp_handle_request)
    ZEND_FE(headers_send, arginfo_headers_send)
    ZEND_FE(frankenphp_finish_request, arginfo_frankenphp_finish_request)
    ZEND_FALIAS(fastcgi_finish_request, frankenphp_finish_request, arginfo_fastcgi_finish_request)
    ZEND_FE(frankenphp_request_headers, arginfo_frankenphp_request_headers)
    ZEND_FALIAS(apache_request_headers, frankenphp_request_headers, arginfo_apache_request_headers)
    ZEND_FALIAS(getallheaders, frankenphp_request_headers, arginfo_getallheaders)
    ZEND_FE(frankenphp_response_headers, arginfo_frankenphp_response_headers)
    ZEND_FALIAS(apache_response_headers, frankenphp_response_headers, arginfo_apache_response_headers)
    ZEND_FE(frankenphp_cache_put, arginfo_frankenphp_cache_put)
    ZEND_FE(frankenphp_cache_get, arginfo_frankenphp_cache_get)
    ZEND_FE(frankenphp_cache_forget, arginfo_frankenphp_cache_forget)
    ZEND_FE_END};
