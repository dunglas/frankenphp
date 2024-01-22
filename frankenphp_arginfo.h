/* This is a generated file, edit the .stub.php file instead.
 * Stub hash: f925a1c280fb955eb32d0823cbd4f360b0cbabed */

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

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_apache_request_headers, 0, 0,
                                        IS_ARRAY, 0)
ZEND_END_ARG_INFO()

#define arginfo_getallheaders arginfo_apache_request_headers

ZEND_FUNCTION(frankenphp_handle_request);
ZEND_FUNCTION(headers_send);
ZEND_FUNCTION(frankenphp_finish_request);
ZEND_FUNCTION(apache_request_headers);

static const zend_function_entry ext_functions[] = {
    ZEND_FE(frankenphp_handle_request, arginfo_frankenphp_handle_request)
        ZEND_FE(headers_send, arginfo_headers_send) ZEND_FE(
            frankenphp_finish_request, arginfo_frankenphp_finish_request)
            ZEND_FALIAS(fastcgi_finish_request, frankenphp_finish_request,
                        arginfo_fastcgi_finish_request)
                ZEND_FE(apache_request_headers, arginfo_apache_request_headers)
                    ZEND_FALIAS(getallheaders, apache_request_headers,
                                arginfo_getallheaders) ZEND_FE_END};
