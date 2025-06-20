#ifndef _{{.HeaderGuard}}
#define _{{.HeaderGuard}}

#include <php.h>
#include <stdint.h>

extern zend_module_entry ext_module_entry;

typedef struct go_value go_value;

typedef struct go_string {
  size_t len;
  char *data;
} go_string;

{{if .Constants}}
/* User defined constants */{{end}}
{{range .Constants}}#define {{.Name}} {{.CValue}}
{{end}}
#endif
