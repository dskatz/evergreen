// Code generated by rest/model/codegen.go. DO NOT EDIT.

package model

import (
	"time"

	"github.com/evergreen-ci/evergreen/rest/model"
)

type APIMockScalars struct {
	TimeType *time.Time             `json:"time_type"`
	MapType  map[string]interface{} `json:"map_type"`
	AnyType  interface{}            `json:"any_type"`
}

func (m *APIMockScalars) BuildFromService(t model.MockScalars) error {
	m.AnyType = InterfaceInterface(t.AnyType)
	m.MapType = MapstringinterfaceMapstringinterface(t.MapType)
	m.TimeType = TimeTimeTimeTimePtr(t.TimeType)
	return nil
}

func (m *APIMockScalars) ToService() (model.MockScalars, error) {
	out := model.MockScalars{}
	out.AnyType = InterfaceInterface(m.AnyType)
	out.MapType = MapstringinterfaceMapstringinterface(m.MapType)
	out.TimeType = TimeTimeTimeTimePtr(m.TimeType)
	return out, nil
}