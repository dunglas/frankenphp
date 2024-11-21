#include <SAPI.h>
#include <Zend/zend_alloc.h>
#include <Zend/zend_exceptions.h>
#include <Zend/zend_interfaces.h>
#include <Zend/zend_types.h>
#include <errno.h>
#include <ext/spl/spl_exceptions.h>
#include <ext/standard/head.h>
#include <inttypes.h>
#include <php.h>
#include <php_config.h>
#include <php_ini.h>
#include <php_main.h>
#include <php_output.h>
#include <php_variables.h>
#include <pthread.h>
#include <sapi/embed/php_embed.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#if defined(__linux__)
#include <sys/prctl.h>
#elif defined(__FreeBSD__) || defined(__OpenBSD__)
#include <pthread_np.h>
#endif

#include "_cgo_export.h"
#include "frankenphp_arginfo.h"

#if defined(PHP_WIN32) && defined(ZTS)
ZEND_TSRMLS_CACHE_DEFINE()
#endif

/* Timeouts are currently fundamentally broken with ZTS except on Linux and
 * FreeBSD: https://bugs.php.net/bug.php?id=79464 */
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
  bool has_main_request;
  bool has_active_request;
  bool worker_ready;
  char *cookie_data;
  bool finished;
} frankenphp_server_context;

__thread bool should_filter_var = 0;
__thread frankenphp_server_context *local_ctx = NULL;
__thread uintptr_t thread_index;
__thread zval *os_environment = NULL;

static void frankenphp_free_request_context() {
  frankenphp_server_context *ctx = SG(server_context);

  free(ctx->cookie_data);
  ctx->cookie_data = NULL;

  /* Is freed via thread.Unpin() at the end of each request */
  SG(request_info).auth_password = NULL;
  SG(request_info).auth_user = NULL;
  SG(request_info).request_method = NULL;
  SG(request_info).query_string = NULL;
  SG(request_info).content_type = NULL;
  SG(request_info).path_translated = NULL;
  SG(request_info).request_uri = NULL;
}

static void frankenphp_destroy_super_globals() {
  zend_try {
    for (int i = 0; i < NUM_TRACK_VARS; i++) {
      zval_ptr_dtor_nogc(&PG(http_globals)[i]);
    }
  }
  zend_end_try();
}

/* Adapted from php_request_shutdown */
static void frankenphp_worker_request_shutdown() {
  /* Flush all output buffers */
  zend_try { php_output_end_all(); }
  zend_end_try();

  /* TODO: store the list of modules to reload in a global module variable */
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

  /* SAPI related shutdown (free stuff) */
  frankenphp_free_request_context();
  zend_try { sapi_deactivate(); }
  zend_end_try();

  zend_set_memory_limit(PG(memory_limit));
  /* TODO: remove next line when https://github.com/php/php-src/pull/14499 will
   * be available */
  SG(rfc1867_uploaded_files) = NULL;
}

PHPAPI void get_full_env(zval *track_vars_array) {
  struct go_getfullenv_return full_env = go_getfullenv(thread_index);

  for (int i = 0; i < full_env.r1; i++) {
    go_string key = full_env.r0[i * 2];
    go_string val = full_env.r0[i * 2 + 1];

    // create PHP strings for key and value
    zend_string *key_str = zend_string_init(key.data, key.len, 0);
    zend_string *val_str = zend_string_init(val.data, val.len, 0);

    // add to the associative array
    add_assoc_str(track_vars_array, ZSTR_VAL(key_str), val_str);

    // release the key string
    zend_string_release(key_str);
  }
}

/* Adapted from php_request_startup() */
static int frankenphp_worker_request_startup() {
  int retval = SUCCESS;

  zend_try {
    frankenphp_destroy_super_globals();
    php_output_activate();

    /* initialize global variables */
    PG(header_is_being_sent) = 0;
    PG(connection_status) = PHP_CONNECTION_NORMAL;

    /* Keep the current execution context */
    sapi_activate();

#ifdef ZEND_MAX_EXECUTION_TIMERS
    if (PG(max_input_time) == -1) {
      zend_set_timeout(EG(timeout_seconds), 1);
    } else {
      zend_set_timeout(PG(max_input_time), 1);
    }
#endif

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

    /* Unfinish the request */
    frankenphp_server_context *ctx = SG(server_context);
    ctx->finished = false;

    /* TODO: store the list of modules to reload in a global module variable */
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

  if (ctx->has_active_request) {
    go_frankenphp_finish_request(thread_index, false);
  }

  ctx->finished = true;

  RETURN_TRUE;
} /* }}} */

/* {{{ Call go's putenv to prevent race conditions */
PHP_FUNCTION(frankenphp_putenv) {
  char *setting;
  size_t setting_len;

  ZEND_PARSE_PARAMETERS_START(1, 1)
  Z_PARAM_STRING(setting, setting_len)
  ZEND_PARSE_PARAMETERS_END();

  // Cast str_len to int (ensure it fits in an int)
  if (setting_len > INT_MAX) {
    php_error(E_WARNING, "String length exceeds maximum integer value");
    RETURN_FALSE;
  }

  if (go_putenv(setting, (int)setting_len)) {
    RETURN_TRUE;
  } else {
    RETURN_FALSE;
  }
} /* }}} */

/* {{{ Call go's getenv to prevent race conditions */
PHP_FUNCTION(frankenphp_getenv) {
  char *name = NULL;
  size_t name_len = 0;
  bool local_only = 0;

  ZEND_PARSE_PARAMETERS_START(0, 2)
  Z_PARAM_OPTIONAL
  Z_PARAM_STRING_OR_NULL(name, name_len)
  Z_PARAM_BOOL(local_only)
  ZEND_PARSE_PARAMETERS_END();

  if (!name) {
    array_init(return_value);
    get_full_env(return_value);

    return;
  }

  go_string gname = {name_len, name};

  struct go_getenv_return result = go_getenv(thread_index, &gname);

  if (result.r0) {
    // Return the single environment variable as a string
    RETVAL_STRINGL(result.r1->data, result.r1->len);
  } else {
    // Environment variable does not exist
    RETVAL_FALSE;
  }
} /* }}} */

/* {{{ Fetch all HTTP request headers */
PHP_FUNCTION(frankenphp_request_headers) {
  if (zend_parse_parameters_none() == FAILURE) {
    RETURN_THROWS();
  }

  frankenphp_server_context *ctx = SG(server_context);
  struct go_apache_request_headers_return headers =
      go_apache_request_headers(thread_index, ctx->has_active_request);

  array_init_size(return_value, headers.r1);

  for (size_t i = 0; i < headers.r1; i++) {
    go_string key = headers.r0[i * 2];
    go_string val = headers.r0[i * 2 + 1];

    add_assoc_stringl_ex(return_value, key.data, key.len, val.data, val.len);
  }
}
/* }}} */

/* add_response_header and apache_response_headers are copied from
 * https://github.com/php/php-src/blob/master/sapi/cli/php_cli_server.c
 * Copyright (c) The PHP Group
 * Licensed under The PHP License
 * Original authors: Moriyoshi Koizumi <moriyoshi@php.net> and Xinchen Hui
 * <laruence@php.net>
 */
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

  if (!ctx->has_main_request) {
    /* not a worker, throw an error */
    zend_throw_exception(
        spl_ce_RuntimeException,
        "frankenphp_handle_request() called while not in worker mode", 0);
    RETURN_THROWS();
  }

  if (!ctx->worker_ready) {
    /* Clean the first dummy request created to initialize the worker */
    frankenphp_worker_request_shutdown();

    ctx->worker_ready = true;
  }

#ifdef ZEND_MAX_EXECUTION_TIMERS
  /* Disable timeouts while waiting for a request to handle */
  zend_unset_timeout();
#endif

  bool request = go_frankenphp_worker_handle_request_start(thread_index);
  if (frankenphp_worker_request_startup() == FAILURE
      /* Shutting down */
      || !request) {
    RETURN_FALSE;
  }

#ifdef ZEND_MAX_EXECUTION_TIMERS
  /*
   * Reset default timeout
   */
  if (PG(max_input_time) != -1) {
    zend_set_timeout(INI_INT("max_execution_time"), 0);
  }
#endif

  /* Call the PHP func */
  zval retval = {0};
  fci.size = sizeof fci;
  fci.retval = &retval;
  if (zend_call_function(&fci, &fcc) == SUCCESS) {
    zval_ptr_dtor(&retval);
  }

  /*
   * If an exception occured, print the message to the client before closing the
   * connection
   */
  if (EG(exception)) {
    zend_exception_error(EG(exception), E_ERROR);
  }

  frankenphp_worker_request_shutdown();
  ctx->has_active_request = false;
  go_frankenphp_finish_request(thread_index, true);

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

PHP_MINIT_FUNCTION(frankenphp) {
  zend_function *func;

  // Override putenv
  func = zend_hash_str_find_ptr(CG(function_table), "putenv",
                                sizeof("putenv") - 1);
  if (func != NULL && func->type == ZEND_INTERNAL_FUNCTION) {
    ((zend_internal_function *)func)->handler = ZEND_FN(frankenphp_putenv);
  } else {
    php_error(E_WARNING, "Failed to find built-in putenv function");
  }

  // Override getenv
  func = zend_hash_str_find_ptr(CG(function_table), "getenv",
                                sizeof("getenv") - 1);
  if (func != NULL && func->type == ZEND_INTERNAL_FUNCTION) {
    ((zend_internal_function *)func)->handler = ZEND_FN(frankenphp_getenv);
  } else {
    php_error(E_WARNING, "Failed to find built-in getenv function");
  }

  return SUCCESS;
}

static zend_module_entry frankenphp_module = {
    STANDARD_MODULE_HEADER,
    "frankenphp",
    ext_functions,         /* function table */
    PHP_MINIT(frankenphp), /* initialization */
    NULL,                  /* shutdown */
    NULL,                  /* request initialization */
    NULL,                  /* request shutdown */
    NULL,                  /* information */
    TOSTRING(FRANKENPHP_VERSION),
    STANDARD_MODULE_PROPERTIES};

static void frankenphp_request_shutdown() {
  frankenphp_server_context *ctx = SG(server_context);

  if (ctx->has_main_request && ctx->has_active_request) {
    frankenphp_destroy_super_globals();
  }

  php_request_shutdown((void *)0);
  frankenphp_free_request_context();

  memset(local_ctx, 0, sizeof(frankenphp_server_context));
}

int frankenphp_update_server_context(
    bool create, bool has_main_request, bool has_active_request,

    const char *request_method, char *query_string, zend_long content_length,
    char *path_translated, char *request_uri, const char *content_type,
    char *auth_user, char *auth_password, int proto_num) {
  frankenphp_server_context *ctx;

  if (create) {
    ctx = local_ctx;

    ctx->worker_ready = false;
    ctx->cookie_data = NULL;
    ctx->finished = false;

    SG(server_context) = ctx;
  } else {
    ctx = (frankenphp_server_context *)SG(server_context);
  }

  // It is not reset by zend engine, set it to 200.
  SG(sapi_headers).http_response_code = 200;

  ctx->has_main_request = has_main_request;
  ctx->has_active_request = has_active_request;

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
  php_import_environment_variables = get_full_env;

  return php_module_startup(sapi_module, &frankenphp_module);
}

static int frankenphp_deactivate(void) {
  /* TODO: flush everything */
  return SUCCESS;
}

static size_t frankenphp_ub_write(const char *str, size_t str_length) {
  frankenphp_server_context *ctx = SG(server_context);

  if (ctx->finished) {
    /* TODO: maybe log a warning that we tried to write to a finished request?
     */
    return 0;
  }

  struct go_ub_write_return result =
      go_ub_write(thread_index, (char *)str, str_length);

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

  if (!ctx->has_active_request) {
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

  go_write_headers(thread_index, status, &sapi_headers->headers);

  return SAPI_HEADER_SENT_SUCCESSFULLY;
}

static void frankenphp_sapi_flush(void *server_context) {
  frankenphp_server_context *ctx = (frankenphp_server_context *)server_context;

  if (ctx && ctx->has_active_request && go_sapi_flush(thread_index)) {
    php_handle_aborted_connection();
  }
}

static size_t frankenphp_read_post(char *buffer, size_t count_bytes) {
  frankenphp_server_context *ctx = SG(server_context);

  return ctx->has_active_request
             ? go_read_post(thread_index, buffer, count_bytes)
             : 0;
}

static char *frankenphp_read_cookies(void) {
  frankenphp_server_context *ctx = SG(server_context);

  if (!ctx->has_active_request) {
    return "";
  }

  ctx->cookie_data = go_read_cookies(thread_index);

  return ctx->cookie_data;
}

/* all variables with well defined keys can safely be registered like this */
void frankenphp_register_trusted_var(zend_string *z_key, char *value,
                                     int val_len, zval *track_vars_array) {
  if (value == NULL) {
    value = "";
  }
  size_t new_val_len = val_len;

  if (!should_filter_var ||
      sapi_module.input_filter(PARSE_SERVER, ZSTR_VAL(z_key), &value,
                               new_val_len, &new_val_len)) {
    zval z_value;
    ZVAL_STRINGL_FAST(&z_value, value, new_val_len);
    zend_hash_update_ind(Z_ARRVAL_P(track_vars_array), z_key, &z_value);
  }
}

/** Persistent strings are ignored by the PHP GC, we have to release these
 * ourselves **/
zend_string *frankenphp_init_persistent_string(const char *string, size_t len) {
  return zend_string_init(string, len, 1);
}

void frankenphp_release_zend_string(zend_string *z_string) {
  zend_string_release(z_string);
}

static void
frankenphp_register_variable_from_request_info(zend_string *zKey, char *value,
                                               bool must_be_present,
                                               zval *track_vars_array) {
  if (value != NULL) {
    frankenphp_register_trusted_var(zKey, value, strlen(value),
                                    track_vars_array);
  } else if (must_be_present) {
    frankenphp_register_trusted_var(zKey, "", 0, track_vars_array);
  }
}

void frankenphp_register_variables_from_request_info(
    zval *track_vars_array, zend_string *content_type,
    zend_string *path_translated, zend_string *query_string,
    zend_string *auth_user, zend_string *request_method,
    zend_string *request_uri) {
  frankenphp_register_variable_from_request_info(
      content_type, (char *)SG(request_info).content_type, false,
      track_vars_array);
  frankenphp_register_variable_from_request_info(
      path_translated, (char *)SG(request_info).path_translated, false,
      track_vars_array);
  frankenphp_register_variable_from_request_info(
      query_string, SG(request_info).query_string, true, track_vars_array);
  frankenphp_register_variable_from_request_info(
      auth_user, (char *)SG(request_info).auth_user, false, track_vars_array);
  frankenphp_register_variable_from_request_info(
      request_method, (char *)SG(request_info).request_method, false,
      track_vars_array);
  frankenphp_register_variable_from_request_info(
      request_uri, SG(request_info).request_uri, true, track_vars_array);
}

/* variables with user-defined keys must be registered safely
 * see: php_variables.c -> php_register_variable_ex (#1106) */
void frankenphp_register_variable_safe(char *key, char *val, size_t val_len,
                                       zval *track_vars_array) {
  if (key == NULL) {
    return;
  }
  if (val == NULL) {
    val = "";
  }
  size_t new_val_len = val_len;
  if (!should_filter_var ||
      sapi_module.input_filter(PARSE_SERVER, key, &val, new_val_len,
                               &new_val_len)) {
    php_register_variable_safe(key, val, new_val_len, track_vars_array);
  }
}

static void frankenphp_register_variables(zval *track_vars_array) {
  /* https://www.php.net/manual/en/reserved.variables.server.php */

  /* In CGI mode, we consider the environment to be a part of the server
   * variables.
   */

  frankenphp_server_context *ctx = SG(server_context);

  /* in non-worker mode we import the os environment regularly */
  if (!ctx->has_main_request) {
    get_full_env(track_vars_array);
    // php_import_environment_variables(track_vars_array);
    go_register_variables(thread_index, track_vars_array);
    return;
  }

  /* In worker mode we cache the os environment */
  if (os_environment == NULL) {
    os_environment = malloc(sizeof(zval));
    array_init(os_environment);
    get_full_env(os_environment);
    // php_import_environment_variables(os_environment);
  }
  zend_hash_copy(Z_ARR_P(track_vars_array), Z_ARR_P(os_environment),
                 (copy_ctor_func_t)zval_add_ref);

  go_register_variables(thread_index, track_vars_array);
}

static void frankenphp_log_message(const char *message, int syslog_type_int) {
  go_log((char *)message, syslog_type_int);
}

static char *frankenphp_getenv(const char *name, size_t name_len) {
  go_string gname = {name_len, (char *)name};

  return go_sapi_getenv(thread_index, &gname);
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
    frankenphp_getenv,     /* getenv */

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

/* Sets thread name for profiling and debugging.
 *
 * Adapted from https://github.com/Pithikos/C-Thread-Pool
 * Copyright: Johan Hanssen Seferidis
 * License: MIT
 */
static void set_thread_name(char *thread_name) {
#if defined(__linux__)
  /* Use prctl instead to prevent using _GNU_SOURCE flag and implicit
   * declaration */
  prctl(PR_SET_NAME, thread_name);
#elif defined(__APPLE__) && defined(__MACH__)
  pthread_setname_np(thread_name);
#elif defined(__FreeBSD__) || defined(__OpenBSD__)
  pthread_set_name_np(pthread_self(), thread_name);
#endif
}

static void *php_thread(void *arg) {
  char thread_name[16] = {0};
  snprintf(thread_name, 16, "php-%" PRIxPTR, (uintptr_t)arg);
  thread_index = (uintptr_t)arg;
  set_thread_name(thread_name);

#ifdef ZTS
  /* initial resource fetch */
  (void)ts_resource(0);
#ifdef PHP_WIN32
  ZEND_TSRMLS_CACHE_UPDATE();
#endif
#endif

  local_ctx = malloc(sizeof(frankenphp_server_context));

  /* check if a default filter is set in php.ini and only filter if
   * it is, this is deprecated and will be removed in PHP 9 */
  char *default_filter;
  cfg_get_string("filter.default", &default_filter);
  should_filter_var = default_filter != NULL;

  while (go_handle_request(thread_index)) {
  }

  go_frankenphp_release_known_variable_keys(thread_index);

#ifdef ZTS
  ts_free_thread();
#endif

  return NULL;
}

static void *php_main(void *arg) {
  /*
   * SIGPIPE must be masked in non-Go threads:
   * https://pkg.go.dev/os/signal#hdr-Go_programs_that_use_cgo_or_SWIG
   */
  sigset_t set;
  sigemptyset(&set);
  sigaddset(&set, SIGPIPE);

  if (pthread_sigmask(SIG_BLOCK, &set, NULL) != 0) {
    perror("failed to block SIGPIPE");
    exit(EXIT_FAILURE);
  }

  intptr_t num_threads = (intptr_t)arg;

  set_thread_name("php-main");

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
  if (frankenphp_sapi_module.ini_entries == NULL) {
    perror("malloc failed");
    exit(EXIT_FAILURE);
  }
  memcpy(frankenphp_sapi_module.ini_entries, HARDCODED_INI,
         sizeof(HARDCODED_INI));
#endif
#endif

  frankenphp_sapi_module.startup(&frankenphp_sapi_module);

  pthread_t *threads = malloc(num_threads * sizeof(pthread_t));
  if (threads == NULL) {
    perror("malloc failed");
    exit(EXIT_FAILURE);
  }

  for (uintptr_t i = 0; i < num_threads; i++) {
    if (pthread_create(&(*(threads + i)), NULL, &php_thread, (void *)i) != 0) {
      perror("failed to create PHP thread");
      free(threads);
      exit(EXIT_FAILURE);
    }
  }

  for (int i = 0; i < num_threads; i++) {
    if (pthread_join((*(threads + i)), NULL) != 0) {
      perror("failed to join PHP thread");
      free(threads);
      exit(EXIT_FAILURE);
    }
  }
  free(threads);

  /* channel closed, shutdown gracefully */
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

  if (pthread_create(&thread, NULL, &php_main, (void *)(intptr_t)num_threads) !=
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

  // free the cached os environment before shutting down the script
  if (os_environment != NULL) {
    zval_ptr_dtor(os_environment);
    free(os_environment);
    os_environment = NULL;
  }

  zend_destroy_file_handle(&file_handle);

  frankenphp_free_request_context();
  frankenphp_request_shutdown();

  return status;
}

/* Use global variables to store CLI arguments to prevent useless allocations */
static char *cli_script;
static int cli_argc;
static char **cli_argv;

/*
 * CLI code is adapted from
 * https://github.com/php/php-src/blob/master/sapi/cli/php_cli.c Copyright (c)
 * The PHP Group Licensed under The PHP License Original uthors: Edin Kadribasic
 * <edink@php.net>, Marcus Boerger <helly@php.net> and Johannes Schlueter
 * <johannes@php.net> Parts based on CGI SAPI Module by Rasmus Lerdorf, Stig
 * Bakken and Zeev Suraski
 */
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

  /*s_in_process = s_in;*/

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

  /*
   * In CGI mode, we consider the environment to be a part of the server
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

  /*
   * The SAPI name "cli" is hardcoded into too many programs... let's usurp it.
   */
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

  /*
   * Start the script in a dedicated thread to prevent conflicts between Go and
   * PHP signal handlers
   */
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

int frankenphp_execute_php_function(const char *php_function) {
  zval retval = {0};
  zend_fcall_info fci = {0};
  zend_fcall_info_cache fci_cache = {0};
  zend_string *func_name =
      zend_string_init(php_function, strlen(php_function), 0);
  ZVAL_STR(&fci.function_name, func_name);
  fci.size = sizeof fci;
  fci.retval = &retval;
  int success = 0;

  zend_try { success = zend_call_function(&fci, &fci_cache) == SUCCESS; }
  zend_end_try();

  zend_string_release(func_name);

  return success;
}

int frankenphp_reset_opcache(void) {
  if (zend_hash_str_exists(CG(function_table), "opcache_reset",
                           sizeof("opcache_reset") - 1)) {
    return frankenphp_execute_php_function("opcache_reset");
  }
  return 0;
}
