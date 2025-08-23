package {{.PackageName}}

/*
#include <stdlib.h>
#include "{{.BaseName}}.h"
*/
import "C"
{{- if .Classes}}
import "runtime/cgo"
{{- end}}
{{- range .Imports}}
import {{.}}
{{- end}}

{{if not .HasInitFunction}}
func init() {
	frankenphp.RegisterExtension(unsafe.Pointer(&C.ext_module_entry))
}
{{end}}
{{range .Constants}}
const {{.Name}} = {{.Value}}
{{- end}}
{{range .InternalFunctions}}
{{.}}
{{- end}}

{{- if .Module}}
{{- if .Module.InitFunc}}
//export {{.Module.InitFunc}}
{{.Module.InitCode}}
{{- end}}

{{- if .Module.ShutdownFunc}}
//export {{.Module.ShutdownFunc}}
{{.Module.ShutdownCode}}
{{- end}}
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
	return h.Value()
}

//export removeGoObject
func removeGoObject(handle C.uintptr_t) {
	h := cgo.Handle(handle)
	h.Delete()
}

{{- end}}

{{- range $class := .Classes}}
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
func {{.Name}}_wrapper(handle C.uintptr_t{{range .Params}}{{if eq .PhpType "string"}}, {{.Name}} *C.zend_string{{else if eq .PhpType "array"}}, {{.Name}} *C.zval{{else}}, {{.Name}} {{if .IsNullable}}*{{end}}{{phpTypeToGoType .PhpType}}{{end}}{{end}}){{if not (isVoid .ReturnType)}}{{if isStringOrArray .ReturnType}} unsafe.Pointer{{else}} {{phpTypeToGoType .ReturnType}}{{end}}{{end}} {
	obj := getGoObject(handle)
	if obj == nil {
{{- if not (isVoid .ReturnType)}}
{{- if isStringOrArray .ReturnType}}
		return nil
{{- else}}
		var zero {{phpTypeToGoType .ReturnType}}
		return zero
{{- end}}
{{- else}}
		return
{{- end}}
	}
	structObj := obj.(*{{$class.GoStruct}})
	{{if not (isVoid .ReturnType)}}return {{end}}structObj.{{.Name | title}}({{range $i, $param := .Params}}{{if $i}}, {{end}}{{$param.Name}}{{end}})
}
{{end}}
{{- end}}
