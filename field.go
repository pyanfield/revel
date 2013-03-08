package revel

import (
	"reflect"
	"strings"
)

// Field represents a data fieid that may be collected in a web form.
type Field struct {
	Name       string
	Error      *ValidationError
	renderArgs map[string]interface{}
}

// A helper for input fields.
// Given a field name, it returns a struct containing the following members:
// Id: the field name, converted to be suitable as a HTML element ID.
// Name: the field name
// Value: the value of the field in the current RenderArgs
// Flash: the flashed value of the field.
// Error: the error message, if any is associated with this field.
// ErrorClass: the raw string “hasError”, if there was an error, else ””.
//
//		{{with $field := field "booking.CheckInDate" .}}
//		  <p class="{{$field.ErrorClass}}">
//		    <strong>Check In Date:</strong>
//		    <input type="text" size="10" name="{{$field.Name}}" class="datepicker" value="{{$field.Flash}}">
//		    * <span class="error">{{$field.Error}}</span>
//		  </p>
		{{end}}
func NewField(name string, renderArgs map[string]interface{}) *Field {
	err, _ := renderArgs["errors"].(map[string]*ValidationError)[name]
	return &Field{
		Name:       name,
		Error:      err,
		renderArgs: renderArgs,
	}
}

// Returns an identifier suitable for use as an HTML id.
func (f *Field) Id() string {
	return strings.Replace(f.Name, ".", "_", -1)
}

// Returned the flashed value of this field.
func (f *Field) Flash() string {
	v, _ := f.renderArgs["flash"].(map[string]string)[f.Name]
	return v
}

// Returned the flashed value of this field as a list.
func (f *Field) FlashArray() []string {
	v := f.Flash()
	if v == "" {
		return []string{}
	}
	return strings.Split(v, ",")
}

// Return the current value of this field.
func (f *Field) Value() interface{} {
	pieces := strings.Split(f.Name, ".")
	answer, ok := f.renderArgs[pieces[0]]
	if !ok {
		return ""
	}

	val := reflect.ValueOf(answer)
	for i := 1; i < len(pieces); i++ {
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		val = val.FieldByName(pieces[i])
		if !val.IsValid() {
			return ""
		}
	}

	return val.Interface()
}

// Return ERROR_CLASS if this field has a validation error, else empty string.
func (f *Field) ErrorClass() string {
	if f.Error != nil {
		return ERROR_CLASS
	}
	return ""
}
