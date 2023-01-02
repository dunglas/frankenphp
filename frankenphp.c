#include <errno.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <php_config.h>
#include <php.h>
#include <php_main.h>
#include <php_variables.h>
#include <php_output.h>
#include <SAPI.h>
#include <Zend/zend_alloc.h>
#include <Zend/zend_types.h>
#include <Zend/zend_exceptions.h>
#include <Zend/zend_interfaces.h>
#include <ext/standard/head.h>

#include "C-Thread-Pool/thpool.h"
#include "C-Thread-Pool/thpool.c"

#include "frankenphp_arginfo.h"
#include "_cgo_export.h"

#if defined(PHP_WIN32) && defined(ZTS)
ZEND_TSRMLS_CACHE_DEFINE()
#endif

/* Timeouts are currently fundamentally broken with ZTS except on Linux: https://bugs.php.net/bug.php?id=79464 */
#ifndef ZEND_TIMER
static const char HARDCODED_INI[] =
	"max_execution_time=0\n"
	"max_input_time=-1\n\0";
#endif

static const char *MODULES_TO_RELOAD[] = {
	"filter",
	"session",
	NULL
};

frankenphp_version frankenphp_get_version() {
	return (frankenphp_version){
		PHP_MAJOR_VERSION,
		PHP_MINOR_VERSION,
		PHP_RELEASE_VERSION,
		PHP_EXTRA_VERSION,
		PHP_VERSION,
		PHP_VERSION_ID,
	};
}

frankenphp_config frankenphp_get_config() {
	return (frankenphp_config){
		frankenphp_get_version(),
#ifdef ZTS
		true,
#else
		false,
#endif
#ifdef ZEND_SIGNALS
		true,
#else
		false,
#endif
#ifdef ZEND_TIMER
		true,
#else
		false,
#endif
	};
}

typedef struct frankenphp_server_context {
	bool worker;
	uintptr_t current_request;
	uintptr_t main_request;				/* Only available during worker initialization */
	char *cookie_data;
	bool finished;
} frankenphp_server_context;

static void frankenphp_request_reset() {
	zend_try {
		int i;

		for (i=0; i<NUM_TRACK_VARS; i++) {
			zval_ptr_dtor(&PG(http_globals)[i]);
		}

		memset(&PG(http_globals), 0, sizeof(zval) * NUM_TRACK_VARS);
	} zend_end_try();
}

/* Adapted from php_request_shutdown */
static void frankenphp_worker_request_shutdown(uintptr_t current_request) {
	/* Flush all output buffers */
	zend_try {
		php_output_end_all();
	} zend_end_try();

	/* Reset max_execution_time (no longer executing php code after response sent) */
	/*zend_try {
		zend_unset_timeout();
	} zend_end_try();*/

	// TODO: store the list of modules to reload in a global module variable
	const char **module_name;
	zend_module_entry *module;
	for (module_name = MODULES_TO_RELOAD; *module_name; module_name++) {
		module = zend_hash_str_find_ptr(&module_registry, *module_name, strlen(*module_name));
		if (module)
			module->request_shutdown_func(module->type, module->module_number);
	}

	/* Shutdown output layer (send the set HTTP headers, cleanup output handlers, etc.) */
	zend_try {
		php_output_deactivate();
	} zend_end_try();

	/* Clean super globals */
	frankenphp_request_reset();

	/* SAPI related shutdown (free stuff) */
	frankenphp_clean_server_context();
	zend_try {
		sapi_deactivate();
	} zend_end_try();

	if (current_request != 0) go_frankenphp_worker_handle_request_end(current_request, true);

	zend_set_memory_limit(PG(memory_limit));

}

/* Adapted from php_request_startup() */
static int frankenphp_worker_request_startup() {
	int retval = SUCCESS;

	zend_try {
		php_output_activate();

		/* initialize global variables */
		PG(header_is_being_sent) = 0;
		PG(connection_status) = PHP_CONNECTION_NORMAL;

		/* Keep the current execution context */
		sapi_activate();

		/*
		 * Timeouts are currently fundamentally broken with ZTS: https://bugs.php.net/bug.php?id=79464
		 *
		 *if (PG(max_input_time) == -1) {
		 *	zend_set_timeout(EG(timeout_seconds), 1);
		 *} else {
		 *	zend_set_timeout(PG(max_input_time), 1);
		 *}
		 */

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

		php_hash_environment();

		zend_is_auto_global(ZSTR_KNOWN(ZEND_STR_AUTOGLOBAL_SERVER));

		// unfinish the request
		frankenphp_server_context *ctx = SG(server_context);
		ctx->finished = false;

		// TODO: store the list of modules to reload in a global module variable
		const char **module_name;
		zend_module_entry *module;
		for (module_name = MODULES_TO_RELOAD; *module_name; module_name++) {
			module = zend_hash_str_find_ptr(&module_registry, *module_name, sizeof(*module_name)-1);
			if (module && module->request_startup_func)
				module->request_startup_func(module->type, module->module_number);
		}
	} zend_catch {
		retval = FAILURE;
	} zend_end_try();

	SG(sapi_started) = 1;

	return retval;
}

PHP_FUNCTION(frankenphp_finish_request) { /* {{{ */
    if (zend_parse_parameters_none() == FAILURE) {
    		RETURN_THROWS();
    }

    frankenphp_server_context *ctx = SG(server_context);

    if(!ctx->finished) {
    	php_output_end_all();
    	php_header();

    	go_frankenphp_worker_handle_request_end(ctx->current_request, false);
    	ctx->finished = true;

    	RETURN_TRUE;
    }

    RETURN_FALSE;

} /* }}} */

PHP_FUNCTION(frankenphp_handle_request) {
	zend_fcall_info fci;
	zend_fcall_info_cache fcc;

	ZEND_PARSE_PARAMETERS_START(1, 1)
		Z_PARAM_FUNC(fci, fcc)
	ZEND_PARSE_PARAMETERS_END();

	frankenphp_server_context *ctx = SG(server_context);

	uintptr_t previous_request = ctx->current_request;
	if (ctx->main_request) {
		/* Clean the first dummy request created to initialize the worker */
		frankenphp_worker_request_shutdown(0);

		previous_request = ctx->main_request;

		/* Mark the worker as ready to handle requests */
		go_frankenphp_worker_ready();
	}

	uintptr_t next_request = go_frankenphp_worker_handle_request_start(previous_request);
	if (
		frankenphp_worker_request_startup() == FAILURE
			/* Shutting down */
			|| !next_request
	) RETURN_FALSE;

	/* Call the PHP func */
	zval retval = {0};
	fci.size = sizeof fci;
	fci.retval = &retval;
	if (zend_call_function(&fci, &fcc) == SUCCESS) {
		zval_ptr_dtor(&retval);
	}

	/* If an exception occured, print the message to the client before closing the connection */
	if (EG(exception))
		zend_exception_error(EG(exception), E_ERROR);

	frankenphp_worker_request_shutdown(next_request);

	RETURN_TRUE;
}

PHP_FUNCTION(headers_send) {
	zend_long response_code = 200;

	ZEND_PARSE_PARAMETERS_START(0, 1)
		Z_PARAM_OPTIONAL
		Z_PARAM_LONG(response_code)
	ZEND_PARSE_PARAMETERS_END();

	int previous_status_code = SG(sapi_headers).http_response_code;
	SG(sapi_headers).http_response_code = response_code;

	if (response_code >= 100 && response_code < 200) {
		int ret = sapi_module.send_headers(&SG(sapi_headers));
		SG(sapi_headers).http_response_code = previous_status_code;

		RETURN_LONG(ret);
	}
	
	RETURN_LONG(sapi_send_headers());
}

static zend_module_entry frankenphp_module = {
    STANDARD_MODULE_HEADER,
    "frankenphp",
    ext_functions,	/* function table */
    NULL,			/* initialization */
    NULL,			/* shutdown */
    NULL,			/* request initialization */
    NULL,			/* request shutdown */
    NULL,			/* information */
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

	return ctx->current_request;
}

uintptr_t frankenphp_request_shutdown()
{
	frankenphp_server_context *ctx = SG(server_context);

	if (ctx->worker && ctx->current_request) {
		frankenphp_request_reset();
	}

	php_request_shutdown((void *) 0);

	free(ctx->cookie_data);
	((frankenphp_server_context*) SG(server_context))->cookie_data = NULL;
	uintptr_t rh = frankenphp_clean_server_context();

	free(ctx);
	SG(server_context) = NULL;

#if defined(ZTS)
	ts_free_thread();
#endif

	return rh;
}

int frankenphp_update_server_context(
	bool create,
	uintptr_t current_request,
	uintptr_t main_request,

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

	if (create) {
#ifdef ZTS
	/* initial resource fetch */
	(void)ts_resource(0);
# ifdef PHP_WIN32
	ZEND_TSRMLS_CACHE_UPDATE();
# endif
#endif

	/* todo: use a pool */
	ctx = (frankenphp_server_context *) calloc(1, sizeof(frankenphp_server_context));
	if (ctx == NULL) return FAILURE;

	ctx->worker = false;
	ctx->current_request = 0;
	ctx->main_request = 0;
	ctx->cookie_data = NULL;
	ctx->finished = false;

	SG(server_context) = ctx;
	} else
		ctx = (frankenphp_server_context *) SG(server_context);

	ctx->main_request = main_request;
	ctx->current_request = current_request;

	if (ctx->main_request) ctx->worker = true;

	SG(request_info).auth_password = auth_password;
	SG(request_info).auth_user = auth_user;
	SG(request_info).request_method = request_method;
	SG(request_info).query_string = query_string;
	SG(request_info).content_type = content_type;
	SG(request_info).content_length = content_length;
	SG(request_info).path_translated = path_translated;
	SG(request_info).request_uri = request_uri;
	SG(request_info).proto_num = proto_num;

	return SUCCESS;
}

static int frankenphp_startup(sapi_module_struct *sapi_module)
{
	return php_module_startup(sapi_module, &frankenphp_module);
}

static int frankenphp_deactivate(void)
{
    /* TODO: flush everything */
    return SUCCESS;
}

static size_t frankenphp_ub_write(const char *str, size_t str_length)
{
	frankenphp_server_context* ctx = SG(server_context);

	if(ctx->finished) {
		// todo: maybe log a warning that we tried to write to a finished request?
		return 0;
	}

	struct go_ub_write_return result = go_ub_write(ctx->current_request ? ctx->current_request : ctx->main_request, (char *) str, str_length);

	if (result.r1) {
		php_handle_aborted_connection();
	}

	return result.r0;
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

	if (ctx->current_request == 0) return SAPI_HEADER_SEND_FAILED;

	h = zend_llist_get_first_ex(&sapi_headers->headers, &pos);
	while (h) {
		go_add_header(ctx->current_request, h->header, h->header_len);
		h = zend_llist_get_next_ex(&sapi_headers->headers, &pos);
	}

	if (!SG(sapi_headers).http_status_line) {
		status = SG(sapi_headers).http_response_code;
		if (!status) status = 200;
	} else {
		status = atoi((SG(sapi_headers).http_status_line) + 9);
	}

	go_write_header(ctx->current_request, status);

	return SAPI_HEADER_SENT_SUCCESSFULLY;
}

static void frankenphp_sapi_flush(void *server_context)
{
	frankenphp_server_context *ctx = (frankenphp_server_context *) server_context;

	if (!ctx || ctx->current_request == 0) return;

	if (go_sapi_flush(ctx->current_request)) php_handle_aborted_connection();
}

static size_t frankenphp_read_post(char *buffer, size_t count_bytes)
{
	frankenphp_server_context* ctx = SG(server_context);

	if (ctx->current_request == 0) return 0;

	return go_read_post(ctx->current_request, buffer, count_bytes);
}

static char* frankenphp_read_cookies(void)
{
	frankenphp_server_context* ctx = SG(server_context);

	if (ctx->current_request == 0) return "";

	ctx->cookie_data = go_read_cookies(ctx->current_request);

	return ctx->cookie_data;
}

void frankenphp_register_bulk_variables(char **variables, size_t size, zval *track_vars_array)
{
	for (size_t i = 0; i < size; i++)
	{
		if (i%2 == 1) php_register_variable(variables[i-1], variables[i], track_vars_array);
	}
}

static void frankenphp_register_variables(zval *track_vars_array)
{
	/* https://www.php.net/manual/en/reserved.variables.server.php */
	frankenphp_server_context* ctx = SG(server_context);

	go_register_variables(ctx->current_request ? ctx->current_request : ctx->main_request, track_vars_array);
}

static void frankenphp_log_message(const char *message, int syslog_type_int)
{
	go_log((char *) message, syslog_type_int);
}

sapi_module_struct frankenphp_sapi_module = {
	"frankenphp",                       /* name */
	"FrankenPHP", 						/* pretty name */

	frankenphp_startup,                 /* startup */
	php_module_shutdown_wrapper,        /* shutdown */

	NULL,                               /* activate */
	frankenphp_deactivate,              /* deactivate */

	frankenphp_ub_write,                /* unbuffered write */
	frankenphp_sapi_flush,              /* flush */
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

static void *manager_thread(void *arg) {
#ifdef ZTS
	// TODO: use tsrm_startup() directly as we now the number of expected threads
	php_tsrm_startup();
	/*tsrm_error_set(TSRM_ERROR_LEVEL_INFO, NULL);*/
# ifdef PHP_WIN32
	ZEND_TSRMLS_CACHE_UPDATE();
# endif
#endif

    sapi_startup(&frankenphp_sapi_module);

#ifndef ZEND_TIMER
	frankenphp_sapi_module.ini_entries = malloc(sizeof(HARDCODED_INI));
	memcpy(frankenphp_sapi_module.ini_entries, HARDCODED_INI, sizeof(HARDCODED_INI));
#endif

	frankenphp_sapi_module.startup(&frankenphp_sapi_module);

	threadpool thpool = thpool_init(*((int *) arg));
	free(arg);

	uintptr_t rh;
	while ((rh = go_fetch_request()))
		thpool_add_work(thpool, go_execute_script, (void *) rh);

	/* channel closed, shutdown gracefully */
	thpool_wait(thpool);
	thpool_destroy(thpool);

	frankenphp_sapi_module.shutdown(&frankenphp_sapi_module);

	sapi_shutdown();
#ifdef ZTS
	tsrm_shutdown();
#endif

	if (frankenphp_sapi_module.ini_entries) {
		free(frankenphp_sapi_module.ini_entries);
		frankenphp_sapi_module.ini_entries = NULL;
	}

	go_shutdown();

	return NULL;
}

int frankenphp_init(int num_threads) {
	pthread_t thread;

	int *num_threads_ptr = calloc(1, sizeof(int));
	*num_threads_ptr = num_threads;

    if (pthread_create(&thread, NULL, *manager_thread, (void *) num_threads_ptr) != 0) {
		go_shutdown();

		return -1;
	}

	return pthread_detach(thread);
}

int frankenphp_request_startup()
{
	if (php_request_startup() == SUCCESS) {
		return SUCCESS;
	}

	frankenphp_server_context *ctx = SG(server_context);
	SG(server_context) = NULL;
	free(ctx);

	php_request_shutdown((void *) 0);

	return FAILURE;
}

int frankenphp_execute_script(const char* file_name)
{
	int status = FAILURE;

	zend_file_handle file_handle;
	zend_stream_init_filename(&file_handle, file_name);
	file_handle.primary_script = 1;

	zend_first_try {
		status = php_execute_script(&file_handle);
	} zend_catch {
    	/* int exit_status = EG(exit_status); */
	} zend_end_try();

	zend_destroy_file_handle(&file_handle);

	return status;
}
