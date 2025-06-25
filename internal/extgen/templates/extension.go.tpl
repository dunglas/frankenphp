package {{.PackageName}}

/*
#include <stdlib.h>
#include "{{.BaseName}}.h"
*/
import "C"
import "runtime/cgo"
{{- range .Imports}}
import {{.}}
{{- end}}

func init() {
	frankenphp.RegisterExtension(unsafe.Pointer(&C.ext_module_entry))
}
{{range .Constants}}
const {{.Name}} = {{.Value}}
{{- end}}
{{range .InternalFunctions}}
{{.}}
{{- end}}

{{- range .Functions}}
//export {{.Name}}
{{.GoFunction}}
{{- end}}

{{- range .Classes}}
type {{.GoStruct}} struct {
{{- range .Properties}}
	{{.Name}} {{.GoType}}
{{- end}}
}
{{- end}}

{{- if .Classes}}

//export registerGoObject
func registerGoObject(obj interface{}) C.uintptr_t {
	handle := cgo.NewHandle(obj)
	return C.uintptr_t(handle)
}

//export getGoObject
func getGoObject(handle C.uintptr_t) interface{} {
	h := cgo.Handle(handle)
	return h.value()
}

//export removeGoObject
func removeGoObject(handle C.uintptr_t) {
	h := cgo.Handle(handle)
	h.Delete()
}

{{- end}}

{{- range .Classes}}
//export create_{{.GoStruct}}_object
func create_{{.GoStruct}}_object() C.uintptr_t {
	obj := &{{.GoStruct}}{}
	return registerGoObject(obj)
}

{{- range .Methods}}
{{- if .GoFunction}}
{{.GoFunction}}
{{- end}}
{{- end}}

{{- range .Methods}}
//export {{.Name}}_wrapper
{{.Wrapper}}
{{end}}
{{- end}}
