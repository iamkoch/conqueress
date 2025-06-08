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

type IGenericIDAggregate[TID any] interface {
	Id() TID
	UncommittedEvents() []cqrs.Event
	ApplyChange(e cqrs.Event)
}

type AggregateRootBase[TID any] struct {
	_changes    []cqrs.Event
	_id         TID
	_version    int
	_innerApply func(e cqrs.Event)
}

func (a *AggregateRootBase[TID]) SetId(id TID) {
	a._id = id
}

func (a *AggregateRootBase[TID]) SetVersion(v int) {
	a._version = v
}

func (a *AggregateRootBase[TID]) SetInnerApply(ia func(e cqrs.Event)) {
	a._innerApply = ia
}

func (a *AggregateRootBase[TID]) InnerApply(e cqrs.Event) {
	a._innerApply(e)
}

type InnerApplier interface {
	InnerApply(e cqrs.Event)
}

type DefaultAggregate[TID any] interface {
	SetBase(base AggregateRootBase[TID])
	GetHandler() func(e cqrs.Event)
	SetInnerApply(ia func(e cqrs.Event))
}

func (a *AggregateRootBase[TID]) Id() TID {
	return a._id
}

func (a *AggregateRootBase[TID]) applyChangeInternal(e cqrs.Event, isNew bool) {
	a.InnerApply(e)

	if isNew {
		a._changes = append(a._changes, e)
	}
}

func (a *AggregateRootBase[TID]) ApplyChange(e cqrs.Event) {
	a.applyChangeInternal(e, true)
}

func (a *AggregateRootBase[TID]) UncommittedEvents() []cqrs.Event {
	return a._changes
}

func NewAggregate[TID any]() AggregateRootBase[TID] {
	return AggregateRootBase[TID]{}
}

func New[T any]() *T {
	n := new(T)
	instance := reflect.New(reflect.TypeOf(n).Elem())
	a := instance.Interface().(DefaultAggregate[guid.Guid])

	a.SetBase(NewAggregate[guid.Guid]())
	a.SetInnerApply(a.GetHandler())
	return reflect.ValueOf(a).Interface().(*T)
}

func NewWithID[T any, TID any]() *T {
	n := new(T)
	instance := reflect.New(reflect.TypeOf(n).Elem())
	a := instance.Interface().(DefaultAggregate[TID])

	a.SetBase(NewAggregate[TID]())
	a.SetInnerApply(a.GetHandler())
	return reflect.ValueOf(a).Interface().(*T)
}

func GetDefaultAggregate[T any]() *T {
	n := new(T)
	instance := reflect.New(reflect.TypeOf(n).Elem())
	a := instance.Interface().(DefaultAggregate[guid.Guid])

	a.SetBase(NewAggregate[guid.Guid]())
	a.SetInnerApply(a.GetHandler())
	return reflect.ValueOf(a).Interface().(*T)
}
