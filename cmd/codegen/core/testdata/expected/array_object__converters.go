// Code generated by rest/model/codegen.go. DO NOT EDIT.

package model

import "github.com/evergreen-ci/evergreen/model/artifact"

func ArrartifactFileArrAPIFile(t []artifact.File) []APIFile {
	m := []APIFile{}
	for _, e := range t {
		m = append(m, *APIFileBuildFromService(e))
	}
	return m
}

func ArrAPIFileArrartifactFile(t []APIFile) []artifact.File {
	m := []artifact.File{}
	for _, e := range t {
		m = append(m, *APIFileToService(e))
	}
	return m
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