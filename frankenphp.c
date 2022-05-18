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

// Helper functions copied from the PHP source code

// main/php_variables.c

static zend_always_inline void php_register_variable_quick(const char *name, size_t name_len, zval *val, HashTable *ht)
{
	zend_string *key = zend_string_init_interned(name, name_len, 0);

	zend_hash_update_ind(ht, key, val);
	zend_string_release_ex(key, 0);
}

static inline void php_register_server_variables(void)
{
	zval tmp;
	zval *arr = &PG(http_globals)[TRACK_VARS_SERVER];
	HashTable *ht;

	zval_ptr_dtor_nogc(arr);
	array_init(arr);

	/* Server variables */
	if (sapi_module.register_server_variables) {
		sapi_module.register_server_variables(arr);
	}
	ht = Z_ARRVAL_P(arr);

	/* PHP Authentication support */
	if (SG(request_info).auth_user) {
		ZVAL_STRING(&tmp, SG(request_info).auth_user);
		php_register_variable_quick("PHP_AUTH_USER", sizeof("PHP_AUTH_USER")-1, &tmp, ht);
	}
	if (SG(request_info).auth_password) {
		ZVAL_STRING(&tmp, SG(request_info).auth_password);
		php_register_variable_quick("PHP_AUTH_PW", sizeof("PHP_AUTH_PW")-1, &tmp, ht);
	}
	if (SG(request_info).auth_digest) {
		ZVAL_STRING(&tmp, SG(request_info).auth_digest);
		php_register_variable_quick("PHP_AUTH_DIGEST", sizeof("PHP_AUTH_DIGEST")-1, &tmp, ht);
	}

	/* store request init time */
	ZVAL_DOUBLE(&tmp, sapi_get_request_time());
	php_register_variable_quick("REQUEST_TIME_FLOAT", sizeof("REQUEST_TIME_FLOAT")-1, &tmp, ht);
	ZVAL_LONG(&tmp, zend_dval_to_lval(Z_DVAL(tmp)));
	php_register_variable_quick("REQUEST_TIME", sizeof("REQUEST_TIME")-1, &tmp, ht);
}

// End of copied functions

typedef struct frankenphp_server_context {
	uintptr_t request;
	uintptr_t requests_chan;
	char *worker_filename;
	char *cookie_data;
} frankenphp_server_context;

// Adapted from php_request_shutdown
void frankenphp_worker_request_shutdown(uintptr_t request) {
	/* 0. skipped: Call any open observer end handlers that are still open after a zend_bailout */
	/* 1. skipped: Call all possible shutdown functions registered with register_shutdown_function() */
	/* 2. skipped: Call all possible __destruct() functions */

	/* 3. Flush all output buffers */
	zend_try {
		php_output_end_all();
	} zend_end_try();

	/* 4. Reset max_execution_time (no longer executing php code after response sent) */
	zend_try {
		zend_unset_timeout();
	} zend_end_try();

	/* 5. Call all extensions RSHUTDOWN functions */
	if (PG(modules_activated)) {
		zend_deactivate_modules();
	}

	/* 6. Shutdown output layer (send the set HTTP headers, cleanup output handlers, etc.) */
	zend_try {
		php_output_deactivate();
	} zend_end_try();
	
	/* 8. Destroy super-globals */
	zend_try {
		int i;

		for (i=0; i<NUM_TRACK_VARS; i++) {
			zval_ptr_dtor(&PG(http_globals)[i]);
		}
	} zend_end_try();

	/* 9. skipped: Shutdown scanner/executor/compiler and restore ini entries */

	/* 10. free request-bound globals */
	// todo: check if it's a good idea
	// php_free_request_globals()
	// clear_last_error()
	if (PG(last_error_message)) {
		zend_string_release(PG(last_error_message));
		PG(last_error_message) = NULL;
	}
	if (PG(last_error_file)) {
		zend_string_release(PG(last_error_file));
		PG(last_error_file) = NULL;
	}
	if (PG(php_sys_temp_dir)) {
		efree(PG(php_sys_temp_dir));
		PG(php_sys_temp_dir) = NULL;
	}

	/* 11. Call all extensions post-RSHUTDOWN functions */
	zend_try {
		zend_post_deactivate_modules();
	} zend_end_try();

	/* 13. skipped: free virtual CWD memory */

	/* 12. SAPI related shutdown (free stuff) */
	frankenphp_clean_server_context();
	zend_try {
		sapi_deactivate();
	} zend_end_try();

	if (request != 0) go_frankenphp_worker_handle_request_end(request);

	/* 14. Destroy stream hashes */
	// todo: check if it's a good idea
	zend_try {
		php_shutdown_stream_hashes();
	} zend_end_try();

	/* 15. skipped: Free Willy (here be crashes) */

	zend_set_memory_limit(PG(memory_limit));
}

// Adapted from php_request_startup()
int frankenphp_worker_request_startup() {
	int retval = SUCCESS;

	zend_try {	
		PG(in_error_log) = 0;
		PG(during_request_startup) = 1;

		php_output_activate();

		/* initialize global variables */
		PG(modules_activated) = 0;
		PG(header_is_being_sent) = 0;
		PG(connection_status) = PHP_CONNECTION_NORMAL;
		PG(in_user_include) = 0;

		// Keep the current execution context
		//zend_activate();
		sapi_activate();

/*#ifdef ZEND_SIGNALS
		zend_signal_activate();
#endif*/

		if (PG(max_input_time) == -1) {
			zend_set_timeout(EG(timeout_seconds), 1);
		} else {
			zend_set_timeout(PG(max_input_time), 1);
		}

		/* Disable realpath cache if an open_basedir is set */
		//if (PG(open_basedir) && *PG(open_basedir)) {
		//	CWDG(realpath_cache_size_limit) = 0;
		//}

		if (PG(expose_php)) {
			sapi_add_header(SAPI_PHP_VERSION_HEADER, sizeof(SAPI_PHP_VERSION_HEADER)-1, 1);
		}

		if (PG(output_handler) && PG(output_handler)[0]) {
			zval oh;

			ZVAL_STRING(&oh, PG(output_handler));
			php_output_start_user(&oh, 0, PHP_OUTPUT_HANDLER_STDFLAGS);
			zval_ptr_dtor(&oh);
		} else if (PG(output_buffering)) {
			php_output_start_user(NULL, PG(output_buffering) > 1 ? PG(output_buffering) : 0, PHP_OUTPUT_HANDLER_STDFLAGS);
		} else if (PG(implicit_flush)) {
			php_output_set_implicit_flush(1);
		}

		PG(during_request_startup) = 0;

		php_hash_environment();

		// todo: find what we need to call php_register_server_variables();
		php_register_server_variables();
		zend_hash_update(&EG(symbol_table), ZSTR_KNOWN(ZEND_STR_AUTOGLOBAL_SERVER), &PG(http_globals)[TRACK_VARS_SERVER]);
		Z_ADDREF(PG(http_globals)[TRACK_VARS_SERVER]);
		HT_ALLOW_COW_VIOLATION(Z_ARRVAL(PG(http_globals)[TRACK_VARS_SERVER]));

		zend_activate_modules();
		PG(modules_activated)=1;
	} zend_catch {
		retval = FAILURE;
	} zend_end_try();

	SG(sapi_started) = 1;

	return retval;
}

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

	if (ctx->request == 0) {
		// Clean the first dummy request created to initialize the worker
		frankenphp_worker_request_shutdown(0);	
	}

	uintptr_t request = go_frankenphp_worker_handle_request_start(ctx->requests_chan);
	if (!request) {
		// Shutting down, re-create a dummy request to make the real php_request_shutdown() function happy
		frankenphp_worker_request_startup();

		RETURN_FALSE;
	}

	if (frankenphp_worker_request_startup() == FAILURE) {
		RETURN_FALSE;
	}

	// Call the PHP func
	zval retval = {0};
	fci.size = sizeof fci;
	fci.retval = &retval;
	zend_call_function(&fci, &fcc);

	frankenphp_worker_request_shutdown(request);

	RETURN_TRUE;
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

uintptr_t frankenphp_clean_server_context() {
	frankenphp_server_context *ctx = SG(server_context);
	if (ctx == NULL) return 0;

	free(SG(request_info.auth_password));
	SG(request_info.auth_password) = NULL;

	free(SG(request_info.auth_user));
	SG(request_info.auth_user) = NULL;

	free((char *) SG(request_info.request_method));
	SG(request_info.request_method) = NULL;

	free(SG(request_info.query_string));
	SG(request_info.query_string) = NULL;

	free((char *) SG(request_info.content_type));
	SG(request_info.content_type) = NULL;

	free(SG(request_info.path_translated));
	SG(request_info.path_translated) = NULL;

	free(SG(request_info.request_uri));
	SG(request_info.request_uri) = NULL;

	return ctx->request;
}

uintptr_t frankenphp_request_shutdown()
{
	php_request_shutdown((void *) 0);

	frankenphp_server_context *ctx = SG(server_context);

	free(ctx->cookie_data);
	((frankenphp_server_context*) SG(server_context))->cookie_data = NULL;

	uintptr_t rh = frankenphp_clean_server_context();

	free(ctx);
	SG(server_context) = NULL;

	return rh;
}

// set worker to 0 if not in worker mode
int frankenphp_create_server_context(uintptr_t requests_chan, char* worker_filename)
{
	frankenphp_server_context *ctx;

	(void) ts_resource(0);

	// todo: use a pool
	ctx = malloc(sizeof(frankenphp_server_context));
	if (ctx == NULL) return FAILURE;

	ctx->request = 0;
	ctx->requests_chan = requests_chan;
	ctx->worker_filename = worker_filename;
	ctx->cookie_data = NULL;

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

	SG(request_info).auth_password = auth_password;
	SG(request_info).auth_user = auth_user;
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
	return php_module_startup(sapi_module, &frankenphp_module, 1);
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

	if (ctx->request == 0) return "";

	ctx->cookie_data = go_read_cookies(ctx->request);

	return ctx->cookie_data;
}

static void frankenphp_register_variables(zval *track_vars_array)
{
	// https://www.php.net/manual/en/reserved.variables.server.php
	frankenphp_server_context* ctx = SG(server_context);

	if (ctx->request == 0 && ctx->worker_filename != NULL) {
		// todo: also register PHP_SELF etc
		php_register_variable_safe("SCRIPT_FILENAME", ctx->worker_filename, strlen(ctx->worker_filename), track_vars_array);
	}

	// todo: import or not environment variables set in the parent process?
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

	fprintf(stderr, "problem in php_request_startup\n");

	php_request_shutdown((void *) 0);
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
    	/* int exit_status = EG(exit_status); */
	} zend_end_try();

	return status;
}
