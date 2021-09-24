#ifndef _FRANKENPPHP_H
#define _FRANKENPPHP_H

int frankenphp_init();
void frankenphp_shutdown();

int frankenphp_request_startup();
int frankenphp_request_shutdown();

int frankenphp_execute_script(const char* file_name);
int frankenphp_request_shutdown();

#endif
