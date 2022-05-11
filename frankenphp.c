#include <errno.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include "_cgo_export.h"
#include "php.h"
#include "SAPI.h"
#include "ext/standard/head.h"
#include "php_main.h"
#include "php_variables.h"
#include "php_output.h"
#include "Zend/zend_alloc.h"

typedef struct frankenphp_server_context {
	uintptr_t request;
	uintptr_t worker;
	char *cookie_data;
} frankenphp_server_context;

ZEND_BEGIN_ARG_INFO_EX(arginfo_frankenphp_handle_request, 0, 0, 1)
    ZEND_ARG_CALLABLE_INFO(false, handler, false)
ZEND_END_ARG_INFO()

PHP_FUNCTION(frankenphp_handle_request) {
	zend_fcall_info fci;
	zend_fcall_info_cache fcc;

	if (zend_parse_parameters(ZEND_NUM_ARGS(), "f", &fci, &fcc) == FAILURE) {
		RETURN_THROWS();
	}

	frankenphp_server_context *ctx = SG(server_context);

	if (go_frankenphp_handle_request(ctx->worker)) {
		RETURN_TRUE;
	}

	RETURN_FALSE;
}

static const zend_function_entry frankenphp_ext_functions[] = {
    PHP_FE(frankenphp_handle_request, arginfo_frankenphp_handle_request)
    PHP_FE_END
};

static zend_module_entry frankenphp_module = {
    STANDARD_MODULE_HEADER,
    "frankenphp",
    frankenphp_ext_functions,    /* function table */
    NULL,  					     /* initialization */
    NULL,                        /* shutdown */
    NULL,                        /* request initialization */
    NULL,                        /* request shutdown */
    NULL,                        /* information */
    "dev",
    STANDARD_MODULE_PROPERTIES
};

void frankenphp_clean_server_context() {
	frankenphp_server_context *ctx = SG(server_context);
	if (ctx == NULL) return;

	sapi_request_info ri = SG(request_info);
	efree(ri.auth_password);
	efree(ri.auth_user);
	free((char *) ri.request_method);
	free(ri.query_string);
	free((char *) ri.content_type);
	free(ri.path_translated);
	free(ri.request_uri);

	if (ctx->request != 0) go_clean_server_context(ctx->request);
}

void frankenphp_request_shutdown()
{
	php_request_shutdown(NULL);

	frankenphp_server_context *ctx = SG(server_context);

	free(ctx->cookie_data);
	frankenphp_clean_server_context();

	free(ctx);
	SG(server_context) = NULL;
}

// set worker to 0 if not in worker mode
int frankenphp_create_server_context(uintptr_t worker)
{
	frankenphp_server_context *ctx;

	(void) ts_resource(0);

	ctx = malloc(sizeof(frankenphp_server_context));
	if (ctx == NULL) {
		return FAILURE;
	}

	ctx->worker = worker;
	ctx->request = 0;

	SG(server_context) = ctx;

	return SUCCESS;
}

void frankenphp_update_server_context(
	uintptr_t request,

	const char *request_method,
	char *query_string,
	zend_long content_length,
	char *path_translated,
	char *request_uri,
	const char *content_type,
	char *auth_user,
	char *auth_password,
	int proto_num
) {
	frankenphp_server_context *ctx = SG(server_context);

	ctx->request = request;

	SG(request_info).auth_password = auth_password == NULL ? NULL : estrdup(auth_password);
	free(auth_password);
	SG(request_info).auth_user = auth_user == NULL ? NULL : estrdup(auth_user);
	free(auth_user);
	SG(request_info).request_method = request_method;
	SG(request_info).query_string = query_string;
	SG(request_info).content_type = content_type;
	SG(request_info).content_length = content_length;
	SG(request_info).path_translated = path_translated;
	SG(request_info).request_uri = request_uri;
	SG(request_info).proto_num = proto_num;
}

static int frankenphp_startup(sapi_module_struct *sapi_module)
{
	return php_module_startup(sapi_module, &frankenphp_module);
}

static int frankenphp_deactivate(void)
{
    // TODO: flush everything
    return SUCCESS;
}

static size_t frankenphp_ub_write(const char *str, size_t str_length)
{
	frankenphp_server_context* ctx = SG(server_context);

	if (ctx->request == 0) return 0; // TODO: write on stdout?

	return go_ub_write(ctx->request, (char *) str, str_length);
}

static int frankenphp_send_headers(sapi_headers_struct *sapi_headers)
{
	if (SG(request_info).no_headers == 1) {
		return SAPI_HEADER_SENT_SUCCESSFULLY;
	}

	sapi_header_struct *h;
	zend_llist_position pos;
	int status;
	frankenphp_server_context* ctx = SG(server_context);

	if (ctx->request == 0) return SAPI_HEADER_SEND_FAILED;

	h = zend_llist_get_first_ex(&sapi_headers->headers, &pos);
	while (h) {
		go_add_header(ctx->request, h->header, h->header_len);
		h = zend_llist_get_next_ex(&sapi_headers->headers, &pos);
	}

	if (!SG(sapi_headers).http_status_line) {
		status = SG(sapi_headers).http_response_code;
		if (!status) status = 200;
	} else {
		status = atoi((SG(sapi_headers).http_status_line) + 9);
	}

	go_write_header(ctx->request, status);

	return SAPI_HEADER_SENT_SUCCESSFULLY;
}

static size_t frankenphp_read_post(char *buffer, size_t count_bytes)
{
	frankenphp_server_context* ctx = SG(server_context);

	if (ctx->request == 0) return 0;

	return go_read_post(ctx->request, buffer, count_bytes);
}

static char* frankenphp_read_cookies(void)
{
	frankenphp_server_context* ctx = SG(server_context);

	if (ctx->request == 0) return NULL;

	ctx->cookie_data = go_read_cookies(ctx->request);

	return ctx->cookie_data;
}

static void frankenphp_register_variables(zval *track_vars_array)
{
	// https://www.php.net/manual/en/reserved.variables.server.php
	frankenphp_server_context* ctx = SG(server_context);

	if (ctx->request == 0) return;

	//php_import_environment_variables(track_vars_array);

	go_register_variables(ctx->request, track_vars_array);
}

static void frankenphp_log_message(const char *message, int syslog_type_int)
{
	// TODO: call Go logger
}

sapi_module_struct frankenphp_sapi_module = {
	"frankenphp",                       /* name */
	"FrankenPHP", 						/* pretty name */

	frankenphp_startup,                 /* startup */
	php_module_shutdown_wrapper,        /* shutdown */

	NULL,                               /* activate */
	frankenphp_deactivate,              /* deactivate */

	frankenphp_ub_write,                /* unbuffered write */
	NULL,                   			/* flush */
	NULL,                               /* get uid */
	NULL,                               /* getenv */

	php_error,                          /* error handler */

	NULL,                               /* header handler */
	frankenphp_send_headers,            /* send headers handler */
	NULL,            				    /* send header handler */

	frankenphp_read_post,               /* read POST data */
	frankenphp_read_cookies,            /* read Cookies */

	frankenphp_register_variables,      /* register server variables */
	frankenphp_log_message,             /* Log message */
	NULL,							    /* Get request time */
	NULL,							    /* Child terminate */

	STANDARD_SAPI_MODULE_PROPERTIES
};

int frankenphp_init() {
    #ifndef ZTS
    return FAILURE;
    #endif

    php_tsrm_startup();
    zend_signal_startup();
    sapi_startup(&frankenphp_sapi_module);

	return frankenphp_sapi_module.startup(&frankenphp_sapi_module);
}

void frankenphp_shutdown()
{
    php_module_shutdown();
	sapi_shutdown();
    tsrm_shutdown();
}

int frankenphp_request_startup()
{
	if (php_request_startup() == SUCCESS) {
		return SUCCESS;
	}

	php_request_shutdown(NULL);
	frankenphp_server_context *ctx = SG(server_context);
	SG(server_context) = NULL;
	free(ctx);

	return FAILURE;
}

int frankenphp_execute_script(const char* file_name)
{
	int status = FAILURE;

	zend_file_handle file_handle;
	zend_stream_init_filename(&file_handle, file_name);

	zend_first_try {
		status = php_execute_script(&file_handle);
	} zend_catch {
    	/* int exit_status = EG(exit_status); */ \
	} zend_end_try();

	return status;
}
