#include <errno.h>
#include "php.h"
#include "SAPI.h"
#include "php_main.h"
#include "php_variables.h"

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
    // TODO: call the Go writer

    return str_length;
}

static void frankenphp_flush(void *server_context)
{
    // TODO: call the Go flusher
}

static void frankenphp_send_header(sapi_header_struct *sapi_header, void *server_context)
{
    // TODO: call the Go set header
}

static char* frankenphp_read_cookies(void)
{
	return NULL;
}

static void frankenphp_register_variables(zval *track_vars_array)
{
	php_import_environment_variables(track_vars_array);
}

static void frankenphp_log_message(const char *message, int syslog_type_int)
{
	// TODO: call Go logger
}

sapi_module_struct frankenphp_module = {
	"frankenphp",                       /* name */
	"FrankenPHP SAPI for Go and Caddy", /* pretty name */

	frankenphp_startup,                 /* startup */
	php_module_shutdown_wrapper,        /* shutdown */

	NULL,                               /* activate */
	frankenphp_deactivate,              /* deactivate */

	frankenphp_ub_write,                /* unbuffered write */
	frankenphp_flush,                   /* flush */
	NULL,                               /* get uid */
	NULL,                               /* getenv */

	php_error,                          /* error handler */

	NULL,                               /* header handler */
	NULL,                               /* send headers handler */
	frankenphp_send_header,             /* send header handler */

	NULL,                               /* read POST data */
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

	// TODO: copied from php_embed.c, check if this is really necessary in our case
	signal(SIGPIPE, SIG_IGN); /* ignore SIGPIPE in standalone mode so
								 that sockets created via fsockopen()
								 don't kill PHP if the remote site
								 closes it.  in apache|apxs mode apache
								 does that for us!  thies@thieso.net
								 20000419 */

    php_tsrm_startup();
# ifdef PHP_WIN32
    ZEND_TSRMLS_CACHE_UPDATE();
# endif

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

int frankenphp_request_startup() {
	if (php_request_startup() == FAILURE) {
		php_request_shutdown(NULL);

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
	php_request_shutdown(NULL);
}
