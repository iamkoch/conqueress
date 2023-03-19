package domain

import (
	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/guid"
	"reflect"
)

type IAggregate interface {
	Id() guid.Guid
	UncommittedEvents() []cqrs.Event
	ApplyChange(e cqrs.Event)
}

type AggregateRootBase struct {
	_changes    []cqrs.Event
	_id         guid.Guid
	_version    int
	_innerApply func(e cqrs.Event)
}

func (a *AggregateRootBase) SetId(id guid.Guid) {
	a._id = id
}

func (a *AggregateRootBase) SetVersion(v int) {
	a._version = v
}

func (a *AggregateRootBase) SetInnerApply(ia func(e cqrs.Event)) {
	a._innerApply = ia
}

func (a *AggregateRootBase) InnerApply(e cqrs.Event) {
	a._innerApply(e)
}

type InnerApplier interface {
	InnerApply(e cqrs.Event)
}

type DefaultAggregate interface {
	SetBase(base AggregateRootBase)
	GetHandler() func(e cqrs.Event)
	SetInnerApply(ia func(e cqrs.Event))
}

func (a *AggregateRootBase) Id() guid.Guid {
	return a._id
}

func (a *AggregateRootBase) applyChangeInternal(e cqrs.Event, isNew bool) {
	a.InnerApply(e)

	if isNew {
		a._changes = append(a._changes, e)
	}
}

func (a *AggregateRootBase) ApplyChange(e cqrs.Event) {
	a.applyChangeInternal(e, true)
}

func (a *AggregateRootBase) UncommittedEvents() []cqrs.Event {
	return a._changes
}

func NewAggregate() AggregateRootBase {
	return AggregateRootBase{}
}

func New[T any]() *T {
	n := new(T)
	instance := reflect.New(reflect.TypeOf(n).Elem())
	a := instance.Interface().(DefaultAggregate)

	a.SetBase(NewAggregate())
	a.SetInnerApply(a.GetHandler())
	return reflect.ValueOf(a).Interface().(*T)
}

func GetDefaultAggregate[T any]() *T {
	n := new(T)
	instance := reflect.New(reflect.TypeOf(n).Elem())
	a := instance.Interface().(DefaultAggregate)

	a.SetBase(NewAggregate())
	a.SetInnerApply(a.GetHandler())
	return reflect.ValueOf(a).Interface().(*T)
}
