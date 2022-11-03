/* This is a generated file, edit the .stub.php file instead.
 * Stub hash: de4dc4063fafd8c933e3068c8349889a7ece5f03 */

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_frankenphp_handle_request, 0, 1, _IS_BOOL, 0)
	ZEND_ARG_TYPE_INFO(0, callback, IS_CALLABLE, 0)
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_headers_send, 0, 0, IS_LONG, 0)
	ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, status, IS_LONG, 0, "200")
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_frankenphp_finish_request, 0, 0, _IS_BOOL, 0)
ZEND_END_ARG_INFO()

#define arginfo_fastcgi_finish_request arginfo_frankenphp_finish_request


ZEND_FUNCTION(frankenphp_handle_request);
ZEND_FUNCTION(headers_send);
ZEND_FUNCTION(frankenphp_finish_request);


static const zend_function_entry ext_functions[] = {
	ZEND_FE(frankenphp_handle_request, arginfo_frankenphp_handle_request)
	ZEND_FE(headers_send, arginfo_headers_send)
	ZEND_FE(frankenphp_finish_request, arginfo_frankenphp_finish_request)
	ZEND_FALIAS(fastcgi_finish_request, frankenphp_finish_request, arginfo_fastcgi_finish_request)
	ZEND_FE_END
};
