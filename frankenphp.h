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

int frankenphp_new_main_thread(int num_threads);
bool frankenphp_new_php_thread(uintptr_t thread_index);

int frankenphp_update_server_context(
    bool create, bool has_main_request, bool has_active_request,

    const char *request_method, char *query_string, zend_long content_length,
    char *path_translated, char *request_uri, const char *content_type,
    char *auth_user, char *auth_password, int proto_num);
int frankenphp_request_startup();
int frankenphp_execute_script(char *file_name);

int frankenphp_execute_script_cli(char *script, int argc, char **argv);

int frankenphp_execute_php_function(const char *php_function);

void frankenphp_register_variables_from_request_info(
    zval *track_vars_array, zend_string *content_type,
    zend_string *path_translated, zend_string *query_string,
    zend_string *auth_user, zend_string *request_method);
void frankenphp_register_variable_safe(char *key, char *var, size_t val_len,
                                       zval *track_vars_array);
zend_string *frankenphp_init_persistent_string(const char *string, size_t len);
void frankenphp_release_zend_string(zend_string *z_string);
int frankenphp_reset_opcache(void);

// clang-format off
void frankenphp_register_bulk(zend_string *remote_addrk, char *remote_addr, size_t remote_addrl,
                              zend_string *remote_hostk, char *remote_host, size_t remote_hostl,
                              zend_string *remote_portk, char *remote_port, size_t remote_portl,
                              zend_string *document_rootk, char *document_root, size_t document_rootl,
                              zend_string *path_infok, char *path_info, size_t path_infol,
                              zend_string *php_selfk, char *php_self, size_t php_selfl,
                              zend_string *document_urik, char *document_uri, size_t document_url,
                              zend_string *script_filenamek, char *script_filename, size_t script_filenamel,
                              zend_string *script_namek, char *script_name, size_t script_namel,
                              zend_string *httpsk, char *https, size_t httpsl,
                              zend_string *ssl_protocolk, char *ssl_protocol, size_t ssl_protocoll,
                              zend_string *request_schemek, char *request_scheme, size_t request_schemel,
                              zend_string *server_namek, char *server_name, size_t server_namel,
                              zend_string *server_portk, char *server_port, size_t server_portl,
                              zend_string *content_lengthk, char *content_length, size_t content_lengthl,
                              zend_string *gateway_interfacek, char *gateway_interface, size_t gateway_interfacel,
                              zend_string *server_protocolk, char *server_protocol, size_t server_protocoll,
                              zend_string *server_softwarek, char *server_software, size_t server_softwarel,
                              zend_string *http_hostk, char *http_host, size_t http_hostl,
                              zend_string *auth_typek, char *auth_type, size_t auth_typel,
                              zend_string *remote_identk, char *remote_ident, size_t remote_identl,
                              zend_string *request_urik, char *request_uri, size_t request_uril,
                              zval *track_vars_array);
// clang-format on

#endif
