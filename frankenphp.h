#ifndef _FRANKENPPHP_H
#define _FRANKENPPHP_H

#include <Zend/zend_types.h>
#include <stdbool.h>
#include <stdint.h>

#ifndef FRANKENPHP_VERSION
#define FRANKENPHP_VERSION dev
#endif
#define STRINGIFY(x) #x
#define TOSTRING(x) STRINGIFY(x)

typedef struct go_string {
  size_t len;
  char *data;
} go_string;

typedef struct php_variable {
  const char *var;
  size_t data_len;
  char *data;
} php_variable;

typedef struct frankenphp_version {
  unsigned char major_version;
  unsigned char minor_version;
  unsigned char release_version;
  const char *extra_version;
  const char *version;
  unsigned long version_id;
} frankenphp_version;
frankenphp_version frankenphp_get_version();

typedef struct frankenphp_config {
  frankenphp_version version;
  bool zts;
  bool zend_signals;
  bool zend_max_execution_timers;
} frankenphp_config;
frankenphp_config frankenphp_get_config();

int frankenphp_init(int num_threads);

int frankenphp_update_server_context(
    bool create, bool has_main_request, bool has_active_request,

    const char *request_method, char *query_string, zend_long content_length,
    char *path_translated, char *request_uri, const char *content_type,
    char *auth_user, char *auth_password, int proto_num);
int frankenphp_request_startup();
int frankenphp_execute_script(char *file_name);
void frankenphp_register_bulk_variables(go_string known_variables[27],
                                        php_variable *dynamic_variables,
                                        size_t size, zval *track_vars_array);

int frankenphp_execute_script_cli(char *script, int argc, char **argv);

int frankenphp_execute_php_function(const char *php_function);

#endif
