#include <errno.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include "_cgo_export.h"
#include "php.h"
#include "SAPI.h"
#include "php_main.h"
#include "php_variables.h"
#include "Zend/zend_alloc.h"

typedef struct frankenphp_server_context {
	uintptr_t response_writer;
	uintptr_t request;
	char *cookie_data;
} frankenphp_server_context;

static int frankenphp_startup(sapi_module_struct *sapi_module)
{
	return php_module_startup(sapi_module, NULL, 0);
}

static int frankenphp_deactivate(void)
{
    // TODO: flush everything
    return SUCCESS;
}

static size_t frankenphp_ub_write(const char *str, size_t str_length)
{
	frankenphp_server_context* ctx = SG(server_context);

	return go_ub_write(ctx->response_writer, (char *) str, str_length);
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

	h = zend_llist_get_first_ex(&sapi_headers->headers, &pos);
	while (h) {
		go_add_header(ctx->response_writer, h->header, h->header_len);
		h = zend_llist_get_next_ex(&sapi_headers->headers, &pos);
	}

	if (!SG(sapi_headers).http_status_line) {
		status = SG(sapi_headers).http_response_code;
		if (!status) status = 200;
	} else {
		status = atoi((SG(sapi_headers).http_status_line) + 9);
	}

	go_write_header(ctx->response_writer, status);

	return SAPI_HEADER_SENT_SUCCESSFULLY;
}

static size_t frankenphp_read_post(char *buffer, size_t count_bytes)
{
	frankenphp_server_context* ctx = SG(server_context);

	return go_read_post(ctx->request, buffer, count_bytes);
}

static char* frankenphp_read_cookies(void)
{
	frankenphp_server_context* ctx = SG(server_context);
	ctx->cookie_data = go_read_cookies(ctx->request);

	return ctx->cookie_data;
}

static void frankenphp_register_variables(zval *track_vars_array)
{
	// https://www.php.net/manual/en/reserved.variables.server.php
	frankenphp_server_context* ctx = SG(server_context);

	//php_import_environment_variables(track_vars_array);

	go_register_variables(ctx->request, track_vars_array);
}

static void frankenphp_log_message(const char *message, int syslog_type_int)
{
	// TODO: call Go logger
}

sapi_module_struct frankenphp_module = {
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
    sapi_startup(&frankenphp_module);

	if (frankenphp_module.startup(&frankenphp_module) == FAILURE) {
		return FAILURE;
	}

    return SUCCESS;
}

void frankenphp_shutdown()
{
    php_module_shutdown();
	sapi_shutdown();
    tsrm_shutdown();
}

int frankenphp_request_startup(
	uintptr_t response_writer,
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
	frankenphp_server_context *ctx;

	(void) ts_resource(0);

	ctx = emalloc(sizeof(frankenphp_server_context));
	if (ctx == NULL) {
		return FAILURE;
	}

	ctx->response_writer = response_writer;
	ctx->request = request;

	SG(server_context) = ctx;

	SG(request_info).request_method = request_method;
	SG(request_info).query_string = query_string;
	SG(request_info).content_length = content_length;
	SG(request_info).path_translated = path_translated;
	SG(request_info).request_uri = request_uri;
	SG(request_info).content_type = content_type;
	if (auth_user != NULL)
		SG(request_info).auth_user = estrdup(auth_user);
	if (auth_password != NULL)
		SG(request_info).auth_password = estrdup(auth_password);
	SG(request_info).proto_num = proto_num;

	if (php_request_startup() == FAILURE) {
		php_request_shutdown(NULL);
		SG(server_context) = NULL;
		free(ctx);

		return FAILURE;
	}

	return SUCCESS;
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

void frankenphp_request_shutdown()
{
	frankenphp_server_context *ctx = SG(server_context);
	php_request_shutdown(NULL);
	if (ctx->cookie_data != NULL) free(ctx->cookie_data);
	efree(ctx);
	SG(server_context) = NULL;
}
