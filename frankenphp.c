#include <SAPI.h>
#include <Zend/zend_alloc.h>
#include <Zend/zend_exceptions.h>
#include <Zend/zend_interfaces.h>
#include <Zend/zend_types.h>
#include <errno.h>
#include <ext/spl/spl_exceptions.h>
#include <ext/standard/head.h>
#include <php.h>
#include <php_config.h>
#include <php_main.h>
#include <php_output.h>
#include <php_variables.h>
#include <sapi/embed/php_embed.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

#include "C-Thread-Pool/thpool.c"
#include "C-Thread-Pool/thpool.h"

#include "_cgo_export.h"
#include "frankenphp_arginfo.h"

#if defined(PHP_WIN32) && defined(ZTS)
ZEND_TSRMLS_CACHE_DEFINE()
#endif

/* Timeouts are currently fundamentally broken with ZTS except on Linux:
 * https://bugs.php.net/bug.php?id=79464 */
#ifndef ZEND_MAX_EXECUTION_TIMERS
static const char HARDCODED_INI[] = "max_execution_time=0\n"
                                    "max_input_time=-1\n\0";
#endif

static const char *MODULES_TO_RELOAD[] = {"filter", "session", NULL};

frankenphp_version frankenphp_get_version() {
  return (frankenphp_version){
      PHP_MAJOR_VERSION, PHP_MINOR_VERSION, PHP_RELEASE_VERSION,
      PHP_EXTRA_VERSION, PHP_VERSION,       PHP_VERSION_ID,
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
#ifdef ZEND_MAX_EXECUTION_TIMERS
      true,
#else
      false,
#endif
  };
}

typedef struct frankenphp_server_context {
  uintptr_t current_request;
  uintptr_t main_request;
  bool worker_ready;
  zval worker_http_globals[6];
  char *cookie_data;
  bool finished;
} frankenphp_server_context;

static uintptr_t frankenphp_clean_server_context() {
  frankenphp_server_context *ctx = SG(server_context);
  if (ctx == NULL) {
    return 0;
  }

  free(SG(request_info).auth_password);
  SG(request_info).auth_password = NULL;

  free(SG(request_info).auth_user);
  SG(request_info).auth_user = NULL;

  free((char *)SG(request_info).request_method);
  SG(request_info).request_method = NULL;

  free(SG(request_info).query_string);
  SG(request_info).query_string = NULL;

  free((char *)SG(request_info).content_type);
  SG(request_info).content_type = NULL;

  free(SG(request_info).path_translated);
  SG(request_info).path_translated = NULL;

  free(SG(request_info).request_uri);
  SG(request_info).request_uri = NULL;

  return ctx->current_request;
}

static void frankenphp_request_reset() {
  zend_try {
    frankenphp_server_context *ctx = SG(server_context);

    int i;
    if (ctx->worker_ready) {
      for (i = 0; i < NUM_TRACK_VARS; i++) {
        zval_ptr_dtor(&PG(http_globals)[i]);
      }

      // Restore worker script super globals
      for (i = 0; i < NUM_TRACK_VARS; i++) {
        ZVAL_COPY(&PG(http_globals)[i], &ctx->worker_http_globals[i]);
      }
      php_hash_environment();
    } else {
      for (i = 0; i < NUM_TRACK_VARS; i++) {
        ZVAL_COPY(&ctx->worker_http_globals[i], &PG(http_globals)[i]);
      }
    }
  }
  zend_end_try();
}

/* Adapted from php_request_shutdown */
static void frankenphp_worker_request_shutdown() {
  /* Flush all output buffers */
  zend_try { php_output_end_all(); }
  zend_end_try();

  // TODO: store the list of modules to reload in a global module variable
  const char **module_name;
  zend_module_entry *module;
  for (module_name = MODULES_TO_RELOAD; *module_name; module_name++) {
    if ((module = zend_hash_str_find_ptr(&module_registry, *module_name,
                                         strlen(*module_name)))) {
      module->request_shutdown_func(module->type, module->module_number);
    }
  }

  /* Shutdown output layer (send the set HTTP headers, cleanup output handlers,
   * etc.) */
  zend_try { php_output_deactivate(); }
  zend_end_try();

  /* Clean super globals */
  frankenphp_request_reset();

  /* SAPI related shutdown (free stuff) */
  frankenphp_clean_server_context();
  zend_try { sapi_deactivate(); }
  zend_end_try();

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
     * Timeouts are currently fundamentally broken with ZTS:
     *https://bugs.php.net/bug.php?id=79464
     *
     *if (PG(max_input_time) == -1) {
     *	zend_set_timeout(EG(timeout_seconds), 1);
     *} else {
     *	zend_set_timeout(PG(max_input_time), 1);
     *}
     */

    if (PG(expose_php)) {
      sapi_add_header(SAPI_PHP_VERSION_HEADER,
                      sizeof(SAPI_PHP_VERSION_HEADER) - 1, 1);
    }

    if (PG(output_handler) && PG(output_handler)[0]) {
      zval oh;

      ZVAL_STRING(&oh, PG(output_handler));
      php_output_start_user(&oh, 0, PHP_OUTPUT_HANDLER_STDFLAGS);
      zval_ptr_dtor(&oh);
    } else if (PG(output_buffering)) {
      php_output_start_user(NULL,
                            PG(output_buffering) > 1 ? PG(output_buffering) : 0,
                            PHP_OUTPUT_HANDLER_STDFLAGS);
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
      if ((module = zend_hash_str_find_ptr(&module_registry, *module_name,
                                           sizeof(*module_name) - 1)) &&
          module->request_startup_func) {
        module->request_startup_func(module->type, module->module_number);
      }
    }
  }
  zend_catch { retval = FAILURE; }
  zend_end_try();

  SG(sapi_started) = 1;

  return retval;
}

PHP_FUNCTION(frankenphp_finish_request) { /* {{{ */
  if (zend_parse_parameters_none() == FAILURE) {
    RETURN_THROWS();
  }

  frankenphp_server_context *ctx = SG(server_context);

  if (ctx->finished) {
    RETURN_FALSE;
  }

  php_output_end_all();
  php_header();

  if (ctx->current_request != 0) {
    go_frankenphp_finish_request(ctx->main_request, ctx->current_request,
                                 false);
  }

  ctx->finished = true;

  RETURN_TRUE;
} /* }}} */

/* {{{ Fetch all HTTP request headers */
PHP_FUNCTION(frankenphp_request_headers) {
  if (zend_parse_parameters_none() == FAILURE) {
    RETURN_THROWS();
  }

  frankenphp_server_context *ctx = SG(server_context);
  struct go_apache_request_headers_return headers =
      go_apache_request_headers(ctx->current_request, ctx->main_request);

  array_init_size(return_value, headers.r1);

  for (size_t i = 0; i < headers.r1; i++) {
    go_string key = headers.r0[i * 2];
    go_string val = headers.r0[i * 2 + 1];

    add_assoc_stringl_ex(return_value, key.data, key.len, val.data, val.len);
  }

  go_apache_request_cleanup(headers.r2);
}
/* }}} */

// add_response_header and apache_response_headers are copied from
// https://github.com/php/php-src/blob/master/sapi/cli/php_cli_server.c
// Copyright (c) The PHP Group
// Licensed under The PHP License
// Original authors: Moriyoshi Koizumi <moriyoshi@php.net> and Xinchen Hui
// <laruence@php.net>
static void add_response_header(sapi_header_struct *h,
                                zval *return_value) /* {{{ */
{
  if (h->header_len > 0) {
    char *s;
    size_t len = 0;
    ALLOCA_FLAG(use_heap)

    char *p = strchr(h->header, ':');
    if (NULL != p) {
      len = p - h->header;
    }
    if (len > 0) {
      while (len != 0 &&
             (h->header[len - 1] == ' ' || h->header[len - 1] == '\t')) {
        len--;
      }
      if (len) {
        s = do_alloca(len + 1, use_heap);
        memcpy(s, h->header, len);
        s[len] = 0;
        do {
          p++;
        } while (*p == ' ' || *p == '\t');
        add_assoc_stringl_ex(return_value, s, len, p,
                             h->header_len - (p - h->header));
        free_alloca(s, use_heap);
      }
    }
  }
}
/* }}} */

PHP_FUNCTION(frankenphp_response_headers) /* {{{ */
{
  if (zend_parse_parameters_none() == FAILURE) {
    RETURN_THROWS();
  }

  array_init(return_value);
  zend_llist_apply_with_argument(
      &SG(sapi_headers).headers,
      (llist_apply_with_arg_func_t)add_response_header, return_value);
}
/* }}} */

PHP_FUNCTION(frankenphp_handle_request) {
  zend_fcall_info fci;
  zend_fcall_info_cache fcc;

  ZEND_PARSE_PARAMETERS_START(1, 1)
  Z_PARAM_FUNC(fci, fcc)
  ZEND_PARSE_PARAMETERS_END();

  frankenphp_server_context *ctx = SG(server_context);

  if (ctx->main_request == 0) {
    // not a worker, throw an error
    zend_throw_exception(
        spl_ce_RuntimeException,
        "frankenphp_handle_request() called while not in worker mode", 0);
    RETURN_THROWS();
  }

  if (!ctx->worker_ready) {
    /* Clean the first dummy request created to initialize the worker */
    frankenphp_worker_request_shutdown();

    ctx->worker_ready = true;

    /* Mark the worker as ready to handle requests */
    go_frankenphp_worker_ready();
  }

#ifdef ZEND_MAX_EXECUTION_TIMERS
  // Disable timeouts while waiting for a request to handle
  zend_unset_timeout();
#endif

  uintptr_t request =
      go_frankenphp_worker_handle_request_start(ctx->main_request);
  if (frankenphp_worker_request_startup() == FAILURE
      /* Shutting down */
      || !request) {
    RETURN_FALSE;
  }

#ifdef ZEND_MAX_EXECUTION_TIMERS
  // Reset default timeout
  // TODO: add support for max_input_time
  zend_set_timeout(INI_INT("max_execution_time"), 0);
#endif

  /* Call the PHP func */
  zval retval = {0};
  fci.size = sizeof fci;
  fci.retval = &retval;
  if (zend_call_function(&fci, &fcc) == SUCCESS) {
    zval_ptr_dtor(&retval);
  }

  /* If an exception occured, print the message to the client before closing the
   * connection */
  if (EG(exception)) {
    zend_exception_error(EG(exception), E_ERROR);
  }

  frankenphp_worker_request_shutdown();
  ctx->current_request = 0;
  go_frankenphp_finish_request(ctx->main_request, request, true);

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
    ext_functions, /* function table */
    NULL,          /* initialization */
    NULL,          /* shutdown */
    NULL,          /* request initialization */
    NULL,          /* request shutdown */
    NULL,          /* information */
    TOSTRING(FRANKENPHP_VERSION),
    STANDARD_MODULE_PROPERTIES};

static uintptr_t frankenphp_request_shutdown() {
  frankenphp_server_context *ctx = SG(server_context);

  if (ctx->main_request && ctx->current_request) {
    frankenphp_request_reset();
  }

  php_request_shutdown((void *)0);

  free(ctx->cookie_data);
  ((frankenphp_server_context *)SG(server_context))->cookie_data = NULL;
  uintptr_t rh = frankenphp_clean_server_context();

  for (int i = 0; i < NUM_TRACK_VARS; i++) {
    zval_dtor(&ctx->worker_http_globals[i]);
  }

  free(ctx);
  SG(server_context) = NULL;
  ctx = NULL;

#if defined(ZTS)
  ts_free_thread();
#endif

  return rh;
}

int frankenphp_update_server_context(
    bool create, uintptr_t current_request, uintptr_t main_request,

    const char *request_method, char *query_string, zend_long content_length,
    char *path_translated, char *request_uri, const char *content_type,
    char *auth_user, char *auth_password, int proto_num) {
  frankenphp_server_context *ctx;

  if (create) {
#ifdef ZTS
    /* initial resource fetch */
    (void)ts_resource(0);
#ifdef PHP_WIN32
    ZEND_TSRMLS_CACHE_UPDATE();
#endif
#endif

    /* todo: use a pool */
    ctx = (frankenphp_server_context *)calloc(
        1, sizeof(frankenphp_server_context));
    if (ctx == NULL) {
      return FAILURE;
    }

    ctx->cookie_data = NULL;
    ctx->finished = false;

    SG(server_context) = ctx;
  } else {
    ctx = (frankenphp_server_context *)SG(server_context);
  }

  ctx->main_request = main_request;
  ctx->current_request = current_request;

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

static int frankenphp_startup(sapi_module_struct *sapi_module) {
  return php_module_startup(sapi_module, &frankenphp_module);
}

static int frankenphp_deactivate(void) {
  /* TODO: flush everything */
  return SUCCESS;
}

static size_t frankenphp_ub_write(const char *str, size_t str_length) {
  frankenphp_server_context *ctx = SG(server_context);

  if (ctx->finished) {
    // TODO: maybe log a warning that we tried to write to a finished request?
    return 0;
  }

  struct go_ub_write_return result = go_ub_write(
      ctx->current_request ? ctx->current_request : ctx->main_request,
      (char *)str, str_length);

  if (result.r1) {
    php_handle_aborted_connection();
  }

  return result.r0;
}

static int frankenphp_send_headers(sapi_headers_struct *sapi_headers) {
  if (SG(request_info).no_headers == 1) {
    return SAPI_HEADER_SENT_SUCCESSFULLY;
  }

  int status;
  frankenphp_server_context *ctx = SG(server_context);

  if (ctx->current_request == 0) {
    return SAPI_HEADER_SEND_FAILED;
  }

  if (SG(sapi_headers).http_status_line) {
    status = atoi((SG(sapi_headers).http_status_line) + 9);
  } else {
    status = SG(sapi_headers).http_response_code;

    if (!status) {
      status = 200;
    }
  }

  go_write_headers(ctx->current_request, status, &sapi_headers->headers);

  return SAPI_HEADER_SENT_SUCCESSFULLY;
}

static void frankenphp_sapi_flush(void *server_context) {
  frankenphp_server_context *ctx = (frankenphp_server_context *)server_context;

  if (ctx && ctx->current_request != 0 && go_sapi_flush(ctx->current_request)) {
    php_handle_aborted_connection();
  }
}

static size_t frankenphp_read_post(char *buffer, size_t count_bytes) {
  frankenphp_server_context *ctx = SG(server_context);

  return ctx->current_request
             ? go_read_post(ctx->current_request, buffer, count_bytes)
             : 0;
}

static char *frankenphp_read_cookies(void) {
  frankenphp_server_context *ctx = SG(server_context);

  if (ctx->current_request == 0) {
    return "";
  }

  ctx->cookie_data = go_read_cookies(ctx->current_request);

  return ctx->cookie_data;
}

static void frankenphp_register_known_variable(const char *key, go_string value,
                                               zval *track_vars_array) {
  if (value.data == NULL) {
    php_register_variable_safe(key, "", 0, track_vars_array);
    return;
  }

  size_t new_val_len;
  if (sapi_module.input_filter(PARSE_SERVER, key, &value.data, value.len,
                               &new_val_len)) {
    php_register_variable_safe(key, value.data, new_val_len, track_vars_array);
  }
}

static void
frankenphp_register_variable_from_request_info(const char *key, char *value,
                                               zval *track_vars_array) {
  if (value == NULL) {
    return;
  }

  size_t new_val_len;
  if (sapi_module.input_filter(PARSE_SERVER, key, &value, strlen(value),
                               &new_val_len)) {
    php_register_variable_safe(key, value, new_val_len, track_vars_array);
  }
}

void frankenphp_register_bulk_variables(go_string known_variables[27],
                                        php_variable *dynamic_variables,
                                        size_t size, zval *track_vars_array) {
  /* Not used, but must be present */
  php_register_variable_safe("AUTH_TYPE", "", 0, track_vars_array);
  php_register_variable_safe("REMOTE_IDENT", "", 0, track_vars_array);

  /* Allocated in frankenphp_update_server_context() */
  frankenphp_register_variable_from_request_info(
      "CONTENT_TYPE", (char *)SG(request_info).content_type, track_vars_array);
  frankenphp_register_variable_from_request_info(
      "PATH_TRANSLATED", (char *)SG(request_info).path_translated,
      track_vars_array);
  frankenphp_register_variable_from_request_info(
      "QUERY_STRING", SG(request_info).query_string, track_vars_array);
  frankenphp_register_variable_from_request_info(
      "REMOTE_USER", (char *)SG(request_info).auth_user, track_vars_array);
  frankenphp_register_variable_from_request_info(
      "REQUEST_METHOD", (char *)SG(request_info).request_method,
      track_vars_array);
  frankenphp_register_variable_from_request_info(
      "REQUEST_URI", SG(request_info).request_uri, track_vars_array);

  /* Known variables */
  frankenphp_register_known_variable("CONTENT_LENGTH", known_variables[0],
                                     track_vars_array);
  frankenphp_register_known_variable("DOCUMENT_ROOT", known_variables[1],
                                     track_vars_array);
  frankenphp_register_known_variable("DOCUMENT_URI", known_variables[2],
                                     track_vars_array);
  frankenphp_register_known_variable("GATEWAY_INTERFACE", known_variables[3],
                                     track_vars_array);
  frankenphp_register_known_variable("HTTP_HOST", known_variables[4],
                                     track_vars_array);
  frankenphp_register_known_variable("HTTPS", known_variables[5],
                                     track_vars_array);
  frankenphp_register_known_variable("PATH_INFO", known_variables[6],
                                     track_vars_array);
  frankenphp_register_known_variable("PHP_SELF", known_variables[7],
                                     track_vars_array);
  frankenphp_register_known_variable("REMOTE_ADDR", known_variables[8],
                                     track_vars_array);
  frankenphp_register_known_variable("REMOTE_HOST", known_variables[9],
                                     track_vars_array);
  frankenphp_register_known_variable("REMOTE_PORT", known_variables[10],
                                     track_vars_array);
  frankenphp_register_known_variable("REQUEST_SCHEME", known_variables[11],
                                     track_vars_array);
  frankenphp_register_known_variable("SCRIPT_FILENAME", known_variables[12],
                                     track_vars_array);
  frankenphp_register_known_variable("SCRIPT_NAME", known_variables[13],
                                     track_vars_array);
  frankenphp_register_known_variable("SERVER_NAME", known_variables[14],
                                     track_vars_array);
  frankenphp_register_known_variable("SERVER_PORT", known_variables[15],
                                     track_vars_array);
  frankenphp_register_known_variable("SERVER_PROTOCOL", known_variables[16],
                                     track_vars_array);
  frankenphp_register_known_variable("SERVER_SOFTWARE", known_variables[17],
                                     track_vars_array);
  frankenphp_register_known_variable("SSL_PROTOCOL", known_variables[18],
                                     track_vars_array);

  size_t new_val_len;
  for (size_t i = 0; i < size; i++) {
    if (sapi_module.input_filter(PARSE_SERVER, dynamic_variables[i].var,
                                 &dynamic_variables[i].data,
                                 dynamic_variables[i].data_len, &new_val_len)) {
      php_register_variable_safe(dynamic_variables[i].var,
                                 dynamic_variables[i].data, new_val_len,
                                 track_vars_array);
    }
  }
}

static void frankenphp_register_variables(zval *track_vars_array) {
  /* https://www.php.net/manual/en/reserved.variables.server.php */
  frankenphp_server_context *ctx = SG(server_context);

  /* In CGI mode, we consider the environment to be a part of the server
   * variables
   */
  php_import_environment_variables(track_vars_array);

  if (ctx->current_request) {
    go_register_variables(ctx->current_request, ctx->main_request,
                          track_vars_array);

    return;
  }

  go_register_variables(ctx->main_request, 0, track_vars_array);
}

static void frankenphp_log_message(const char *message, int syslog_type_int) {
  go_log((char *)message, syslog_type_int);
}

sapi_module_struct frankenphp_sapi_module = {
    "frankenphp", /* name */
    "FrankenPHP", /* pretty name */

    frankenphp_startup,          /* startup */
    php_module_shutdown_wrapper, /* shutdown */

    NULL,                  /* activate */
    frankenphp_deactivate, /* deactivate */

    frankenphp_ub_write,   /* unbuffered write */
    frankenphp_sapi_flush, /* flush */
    NULL,                  /* get uid */
    NULL,                  /* getenv */

    php_error, /* error handler */

    NULL,                    /* header handler */
    frankenphp_send_headers, /* send headers handler */
    NULL,                    /* send header handler */

    frankenphp_read_post,    /* read POST data */
    frankenphp_read_cookies, /* read Cookies */

    frankenphp_register_variables, /* register server variables */
    frankenphp_log_message,        /* Log message */
    NULL,                          /* Get request time */
    NULL,                          /* Child terminate */

    STANDARD_SAPI_MODULE_PROPERTIES};

static void *manager_thread(void *arg) {
  // SIGPIPE must be masked in non-Go threads:
  // https://pkg.go.dev/os/signal#hdr-Go_programs_that_use_cgo_or_SWIG
  sigset_t set;
  sigemptyset(&set);
  sigaddset(&set, SIGPIPE);

  if (pthread_sigmask(SIG_BLOCK, &set, NULL) != 0) {
    perror("failed to block SIGPIPE");
    exit(EXIT_FAILURE);
  }

  int num_threads = *((int *)arg);
  free(arg);
  arg = NULL;

#ifdef ZTS
#if (PHP_VERSION_ID >= 80300)
  php_tsrm_startup_ex(num_threads);
#else
  php_tsrm_startup();
#endif
  /*tsrm_error_set(TSRM_ERROR_LEVEL_INFO, NULL);*/
#ifdef PHP_WIN32
  ZEND_TSRMLS_CACHE_UPDATE();
#endif
#endif

  sapi_startup(&frankenphp_sapi_module);

#ifndef ZEND_MAX_EXECUTION_TIMERS
#if (PHP_VERSION_ID >= 80300)
  frankenphp_sapi_module.ini_entries = HARDCODED_INI;
#else
  frankenphp_sapi_module.ini_entries = malloc(sizeof(HARDCODED_INI));
  memcpy(frankenphp_sapi_module.ini_entries, HARDCODED_INI,
         sizeof(HARDCODED_INI));
#endif
#endif

  frankenphp_sapi_module.startup(&frankenphp_sapi_module);

  threadpool thpool = thpool_init(num_threads);

  uintptr_t rh;
  while ((rh = go_fetch_request())) {
    thpool_add_work(thpool, go_execute_script, (void *)rh);
  }

  /* channel closed, shutdown gracefully */
  thpool_wait(thpool);
  thpool_destroy(thpool);

  frankenphp_sapi_module.shutdown(&frankenphp_sapi_module);

  sapi_shutdown();
#ifdef ZTS
  tsrm_shutdown();
#endif

#if (PHP_VERSION_ID < 80300)
  if (frankenphp_sapi_module.ini_entries) {
    free(frankenphp_sapi_module.ini_entries);
    frankenphp_sapi_module.ini_entries = NULL;
  }
#endif

  go_shutdown();

  return NULL;
}

int frankenphp_init(int num_threads) {
  pthread_t thread;

  int *num_threads_ptr = calloc(1, sizeof(int));
  *num_threads_ptr = num_threads;

  if (pthread_create(&thread, NULL, *manager_thread, (void *)num_threads_ptr) !=
      0) {
    go_shutdown();

    return -1;
  }

  return pthread_detach(thread);
}

int frankenphp_request_startup() {
  if (php_request_startup() == SUCCESS) {
    return SUCCESS;
  }

  frankenphp_server_context *ctx = SG(server_context);
  SG(server_context) = NULL;
  free(ctx);
  ctx = NULL;

  php_request_shutdown((void *)0);

  return FAILURE;
}

int frankenphp_execute_script(char *file_name) {
  if (frankenphp_request_startup() == FAILURE) {
    free(file_name);
    file_name = NULL;

    return FAILURE;
  }

  int status = SUCCESS;

  zend_file_handle file_handle;
  zend_stream_init_filename(&file_handle, file_name);
  free(file_name);
  file_name = NULL;

  file_handle.primary_script = 1;

  zend_first_try {
    EG(exit_status) = 0;
    php_execute_script(&file_handle);
    status = EG(exit_status);
  }
  zend_catch { status = EG(exit_status); }
  zend_end_try();

  zend_destroy_file_handle(&file_handle);

  frankenphp_clean_server_context();
  frankenphp_request_shutdown();

  return status;
}

// Use global variables to store CLI arguments to prevent useless allocations
static char *cli_script;
static int cli_argc;
static char **cli_argv;

// CLI code is adapted from
// https://github.com/php/php-src/blob/master/sapi/cli/php_cli.c Copyright (c)
// The PHP Group Licensed under The PHP License Original uthors: Edin Kadribasic
// <edink@php.net>, Marcus Boerger <helly@php.net> and Johannes Schlueter
// <johannes@php.net> Parts based on CGI SAPI Module by Rasmus Lerdorf, Stig
// Bakken and Zeev Suraski
static void cli_register_file_handles(bool no_close) /* {{{ */
{
  php_stream *s_in, *s_out, *s_err;
  php_stream_context *sc_in = NULL, *sc_out = NULL, *sc_err = NULL;
  zend_constant ic, oc, ec;

  s_in = php_stream_open_wrapper_ex("php://stdin", "rb", 0, NULL, sc_in);
  s_out = php_stream_open_wrapper_ex("php://stdout", "wb", 0, NULL, sc_out);
  s_err = php_stream_open_wrapper_ex("php://stderr", "wb", 0, NULL, sc_err);

  if (s_in == NULL || s_out == NULL || s_err == NULL) {
    if (s_in)
      php_stream_close(s_in);
    if (s_out)
      php_stream_close(s_out);
    if (s_err)
      php_stream_close(s_err);
    return;
  }

  if (no_close) {
    s_in->flags |= PHP_STREAM_FLAG_NO_CLOSE;
    s_out->flags |= PHP_STREAM_FLAG_NO_CLOSE;
    s_err->flags |= PHP_STREAM_FLAG_NO_CLOSE;
  }

  // s_in_process = s_in;

  php_stream_to_zval(s_in, &ic.value);
  php_stream_to_zval(s_out, &oc.value);
  php_stream_to_zval(s_err, &ec.value);

  ZEND_CONSTANT_SET_FLAGS(&ic, CONST_CS, 0);
  ic.name = zend_string_init_interned("STDIN", sizeof("STDIN") - 1, 0);
  zend_register_constant(&ic);

  ZEND_CONSTANT_SET_FLAGS(&oc, CONST_CS, 0);
  oc.name = zend_string_init_interned("STDOUT", sizeof("STDOUT") - 1, 0);
  zend_register_constant(&oc);

  ZEND_CONSTANT_SET_FLAGS(&ec, CONST_CS, 0);
  ec.name = zend_string_init_interned("STDERR", sizeof("STDERR") - 1, 0);
  zend_register_constant(&ec);
}
/* }}} */

static void sapi_cli_register_variables(zval *track_vars_array) /* {{{ */
{
  size_t len;
  char *docroot = "";

  /* In CGI mode, we consider the environment to be a part of the server
   * variables
   */
  php_import_environment_variables(track_vars_array);

  /* Build the special-case PHP_SELF variable for the CLI version */
  len = strlen(cli_script);
  if (sapi_module.input_filter(PARSE_SERVER, "PHP_SELF", &cli_script, len,
                               &len)) {
    php_register_variable_safe("PHP_SELF", cli_script, len, track_vars_array);
  }
  if (sapi_module.input_filter(PARSE_SERVER, "SCRIPT_NAME", &cli_script, len,
                               &len)) {
    php_register_variable_safe("SCRIPT_NAME", cli_script, len,
                               track_vars_array);
  }
  /* filenames are empty for stdin */
  if (sapi_module.input_filter(PARSE_SERVER, "SCRIPT_FILENAME", &cli_script,
                               len, &len)) {
    php_register_variable_safe("SCRIPT_FILENAME", cli_script, len,
                               track_vars_array);
  }
  if (sapi_module.input_filter(PARSE_SERVER, "PATH_TRANSLATED", &cli_script,
                               len, &len)) {
    php_register_variable_safe("PATH_TRANSLATED", cli_script, len,
                               track_vars_array);
  }
  /* just make it available */
  len = 0U;
  if (sapi_module.input_filter(PARSE_SERVER, "DOCUMENT_ROOT", &docroot, len,
                               &len)) {
    php_register_variable_safe("DOCUMENT_ROOT", docroot, len, track_vars_array);
  }
}
/* }}} */

static void *execute_script_cli(void *arg) {
  void *exit_status;

  // The SAPI name "cli" is hardcoded into too many programs... let's usurp it.
  php_embed_module.name = "cli";
  php_embed_module.pretty_name = "PHP CLI embedded in FrankenPHP";
  php_embed_module.register_server_variables = sapi_cli_register_variables;

  php_embed_init(cli_argc, cli_argv);

  cli_register_file_handles(false);
  zend_first_try {
    zend_file_handle file_handle;
    zend_stream_init_filename(&file_handle, cli_script);

    CG(skip_shebang) = 1;
    php_execute_script(&file_handle);
  }
  zend_end_try();

  exit_status = (void *)(intptr_t)EG(exit_status);

  php_embed_shutdown();

  return exit_status;
}

int frankenphp_execute_script_cli(char *script, int argc, char **argv) {
  pthread_t thread;
  int err;
  void *exit_status;

  cli_script = script;
  cli_argc = argc;
  cli_argv = argv;

  // Start the script in a dedicated thread to prevent conflicts between Go and
  // PHP signal handlers
  err = pthread_create(&thread, NULL, execute_script_cli, NULL);
  if (err != 0) {
    return err;
  }

  err = pthread_join(thread, &exit_status);
  if (err != 0) {
    return err;
  }

  return (intptr_t)exit_status;
}
