package store

import (
	"fmt"
	"reflect"
)

type TypeMap struct {
	typeMap map[string]reflect.Type
}

var ErrTypeNotFound = fmt.Errorf("type not found")

func (tm *TypeMap) Get(t string) (reflect.Type, error) {
	if match, exist := tm.typeMap[t]; exist {
		return match, nil
	}
	return nil, ErrTypeNotFound
}

func NewTypeMap() *TypeMap {
	return &TypeMap{typeMap: make(map[string]reflect.Type)}
}

func (tm *TypeMap) Add(t interface{}) *TypeMap {
	tm.typeMap[reflect.TypeOf(t).Name()] = reflect.TypeOf(t)
	return tm
}
