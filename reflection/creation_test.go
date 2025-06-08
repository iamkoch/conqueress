package reflection

import (
	"reflect"
	"testing"
)

// Example structs for testing
type TestStruct struct {
	Field1 string
	Field2 int
}

type TestStructWithPointer struct {
	Field1 *string
	Field2 *int
}

// TestCreateInstanceForNonPointerType tests CreateInstance with a non-pointer type
func TestCreateInstanceForNonPointerType(t *testing.T) {
	instance := CreateInstance[TestStruct]()
	if reflect.TypeOf(instance).Kind() != reflect.Struct {
		t.Errorf("expected a struct, got %T", instance)
	}

	// Check if fields are initialized to zero values
	if instance.Field1 != "" || instance.Field2 != 0 {
		t.Errorf("fields were not initialized to zero values: %+v", instance)
	}
}

// TestCreateInstanceForPointerType tests CreateInstance with a pointer type
func TestCreateInstanceForPointerType(t *testing.T) {
	instance := CreateInstance[*TestStruct]()
	if reflect.TypeOf(instance).Kind() != reflect.Ptr {
		t.Errorf("expected a pointer, got %T", instance)
	}

	// Check if the pointer is non-nil
	if instance == nil {
		t.Errorf("expected a non-nil pointer, got nil")
	}

	// Check if the pointed-to value is initialized to zero values
	if instance.Field1 != "" || instance.Field2 != 0 {
		t.Errorf("fields were not initialized to zero values: %+v", instance)
	}
}

// TestCreateInstanceForStructWithPointerFields tests CreateInstance with a struct containing pointer fields
func TestCreateInstanceForStructWithPointerFields(t *testing.T) {
	instance := CreateInstance[TestStructWithPointer]()
	if reflect.TypeOf(instance).Kind() != reflect.Struct {
		t.Errorf("expected a struct, got %T", instance)
	}

	// Check if fields are initialized to nil
	if instance.Field1 != nil || instance.Field2 != nil {
		t.Errorf("pointer fields were not initialized to nil: %+v", instance)
	}
}

// TestCreateInstanceForPointerToStructWithPointerFields tests CreateInstance with a pointer to a struct containing pointer fields
func TestCreateInstanceForPointerToStructWithPointerFields(t *testing.T) {
	instance := CreateInstance[*TestStructWithPointer]()
	if reflect.TypeOf(instance).Kind() != reflect.Ptr {
		t.Errorf("expected a pointer, got %T", instance)
	}

	// Check if the pointer is non-nil
	if instance == nil {
		t.Errorf("expected a non-nil pointer, got nil")
	}

	// Check if the pointed-to fields are initialized to nil
	if instance.Field1 != nil || instance.Field2 != nil {
		t.Errorf("pointer fields were not initialized to nil: %+v", instance)
	}
}

// TestCreateInstanceForBasicType tests CreateInstance with a basic type (should fail compilation)
// Uncommenting this test will cause a compilation error, as `T` must be a type, not a basic type
/*
func TestCreateInstanceForBasicType(t *testing.T) {
	instance := CreateInstance[int]()
	if instance != 0 {
		t.Errorf("expected zero value for int, got %v", instance)
	}
}
*/
