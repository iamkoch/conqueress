package conqueress

import (
	"github.com/iamkoch/conqueress/guid"
	"reflect"
)

type Event interface {
	MsgId() guid.Guid
	WithVersion(v int)
	Version() int
}

type BaseEvent struct {
	MessageId string `json:"message_id"`
	Ver       int    `json:"version"`
}

func (b *BaseEvent) Version() int {
	return b.Ver
}

func (b *BaseEvent) MsgId() guid.Guid {
	return guid.MustFromString(b.MessageId)
}

func (b *BaseEvent) WithVersion(v int) {
	b.Ver = v
}

func defaultBaseEvent() *BaseEvent {
	return &BaseEvent{Ver: -1, MessageId: guid.New().String()}
}

// NewEvent creates a new instance of the specified type T and populates its BaseEvent field if present and settable.
// T cannot be a pointer
func NewEvent[T any](setters ...func(*T)) T {
	var t T

	// Determine if T is a pointer type
	tType := reflect.TypeOf(t)
	isPtr := tType.Kind() == reflect.Ptr

	if isPtr {
		panic("T cannot be a pointer")
	}

	// Create an instance of T
	var tValue reflect.Value
	// Create a new instance of T as a value
	tValue = reflect.New(tType).Elem()

	// Initialize the BaseEvent
	baseEvent := defaultBaseEvent()

	// Access the underlying value (for setting fields)
	tValueElem := tValue

	// Iterate over fields of T and set BaseEvent if applicable
	for i := 0; i < tValueElem.NumField(); i++ {
		field := tValueElem.Type().Field(i)
		if field.Name == "BaseEvent" {
			fieldValue := tValueElem.FieldByName("BaseEvent")
			if fieldValue.CanSet() {
				if field.Type.Kind() == reflect.Ptr {
					fieldValue.Set(reflect.ValueOf(baseEvent))
				} else {
					fieldValue.Set(reflect.ValueOf(*baseEvent))
				}
			}
		}
	}

	if len(setters) == 0 {
		return tValue.Interface().(T)
	}

	outputValue := tValue.Interface().(T)
	for _, setter := range setters {
		setter(&outputValue)
	}

	return outputValue
}
