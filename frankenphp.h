#ifndef _FRANKENPPHP_H
#define _FRANKENPPHP_H

#include <stdint.h>

int frankenphp_init();
void frankenphp_shutdown();

int frankenphp_create_server_context(uintptr_t worker);
void frankenphp_update_server_context(
	uintptr_t request,

	const char *request_method,
	char *query_string,
	int64_t content_length,
	char *path_translated,
	char *request_uri,
	const char *content_type,
	char *auth_user,
	char *auth_password,
	int proto_num
);
void frankenphp_clean_server_context();
int frankenphp_request_startup();
int frankenphp_execute_script(const char* file_name);
void frankenphp_request_shutdown();

#endif
