// Code generated by rest/model/codegen.go. DO NOT EDIT.

package model

func ArrstringArrstring(t []string) []string {
	m := []string{}
	for _, e := range t {
		m = append(m, StringString(e))
	}
	return m
}

func BoolBool(in bool) bool {
	return bool(in)
}

func BoolBoolPtr(in bool) *bool {
	out := bool(in)
	return &out
}

func BoolPtrBool(in *bool) bool {
	var out bool
	if in == nil {
		return out
	}
	return bool(*in)
}

func BoolPtrBoolPtr(in *bool) *bool {
	if in == nil {
		return nil
	}
	out := bool(*in)
	return &out
}

func StringString(in string) string {
	return string(in)
}

func StringStringPtr(in string) *string {
	out := string(in)
	return &out
}

func StringPtrString(in *string) string {
	var out string
	if in == nil {
		return out
	}
	return string(*in)
}

func StringPtrStringPtr(in *string) *string {
	if in == nil {
		return nil
	}
	out := string(*in)
	return &out
}