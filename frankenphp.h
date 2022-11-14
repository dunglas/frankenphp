#ifndef _FRANKENPPHP_H
#define _FRANKENPPHP_H

#include <stdint.h>
#include <Zend/zend_types.h>

typedef struct frankenphp_php_version {
	int major_version;
	int minor_version;
	int release_version;
	const char *extra_version;
	const char *version;
	int version_id;
} frankenphp_php_version;

frankenphp_php_version frankenphp_version();
int frankenphp_check_version();
int frankenphp_init(int num_threads);

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
);
int frankenphp_worker_reset_server_context();
uintptr_t frankenphp_clean_server_context();
int frankenphp_request_startup();
int frankenphp_execute_script(const char *file_name);
uintptr_t frankenphp_request_shutdown();
void frankenphp_register_bulk_variables(char **variables, size_t size, zval *track_vars_array);

#endif
