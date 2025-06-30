#ifndef _{{.HeaderGuard}}
#define _{{.HeaderGuard}}

#include <php.h>
#include <stdint.h>

extern zend_module_entry ext_module_entry;

{{if .Constants}}
/* User defined constants */{{end}}
{{range .Constants}}#define {{.Name}} {{.CValue}}
{{end}}
#endif
