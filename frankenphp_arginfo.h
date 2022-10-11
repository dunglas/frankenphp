/* This is a generated file, edit the .stub.php file instead.
 * Stub hash: f9ead962eae043fa397a4e573e8905876b7b390b */

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_frankenphp_handle_request, 0, 1, _IS_BOOL, 0)
	ZEND_ARG_TYPE_INFO(0, callback, IS_CALLABLE, 0)
ZEND_END_ARG_INFO()

ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_headers_send, 0, 0, IS_LONG, 0)
	ZEND_ARG_TYPE_INFO_WITH_DEFAULT_VALUE(0, status, IS_LONG, 0, "200")
ZEND_END_ARG_INFO()


ZEND_FUNCTION(frankenphp_handle_request);
ZEND_FUNCTION(headers_send);


static const zend_function_entry ext_functions[] = {
	ZEND_FE(frankenphp_handle_request, arginfo_frankenphp_handle_request)
	ZEND_FE(headers_send, arginfo_headers_send)
	ZEND_FE_END
};
