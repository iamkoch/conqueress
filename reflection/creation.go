package reflection

import "reflect"

// CreateInstance generates a new instance of the generic type T and determines if T is a pointer or value type.
// Returns the created instance and a boolean indicating if T is a pointer type.
func CreateInstance[T any]() (T, bool) {
	var t T

	// Determine if T is a pointer type
	tType := reflect.TypeOf(t)
	isPtr := tType.Kind() == reflect.Ptr

	// Create an instance of T
	var tValue reflect.Value
	if isPtr {
		// Create a new pointer to T and dereference it
		tValue = reflect.New(tType.Elem())
	} else {
		// Create a new instance of T as a value
		tValue = reflect.New(tType).Elem()
	}

	return tValue.Interface().(T), isPtr
}
