package validator

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "validation failed"
	}
	return fmt.Sprintf("validation failed: %s %s", e[0].Field, e[0].Message)
}

func ValidateStruct(value any) error {
	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return errors.New("validate nil value")
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return errors.New("validate nil pointer")
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return errors.New("validate expects a struct")
	}

	t := v.Type()
	var errs ValidationErrors
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		rules := field.Tag.Get("validate")
		if rules == "" || rules == "-" {
			continue
		}
		name := jsonName(field)
		value := v.Field(i)
		for _, rule := range strings.Split(rules, ",") {
			switch strings.TrimSpace(rule) {
			case "required":
				if isZero(value) {
					errs = append(errs, ValidationError{Field: name, Message: "is required"})
				}
			}
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func jsonName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return field.Name
	}
	name := strings.Split(tag, ",")[0]
	if name == "" || name == "-" {
		return field.Name
	}
	return name
}

func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Bool:
		return false
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	case reflect.Struct:
		return v.IsZero()
	default:
		return v.IsZero()
	}
}
