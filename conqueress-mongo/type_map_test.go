package store

import (
	enshur "github.com/iamkoch/ensure/stateless"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestTypeMap(t *testing.T) {
	var (
		typeMap *TypeMap
		assert  = assert.New(t)
		actual  reflect.Type
		getErr  error
	)
	enshur.That("typemap works as expected", func(s *enshur.Scenario) {
		s.Given("a typemap", func() {
			typeMap = NewTypeMap()
		})

		s.And("some registered types", func() {
			typeMap.Add(TypeOne{}).Add(TypeTwo{})
		})

		s.When("I request an existing type", func() {
			actual, getErr = typeMap.Get("TypeOne")
		})

		s.Then("I should not get an error", func() {
			assert.Nil(getErr)
		})

		s.Then("I should get the correct type", func() {
			expectedType := reflect.TypeOf(TypeOne{})
			assert.Equal(expectedType, actual)
		})

		s.When("I request a non-existent type", func() {
			actual, getErr = typeMap.Get("TypeThree")
		})

		s.Then("I should get an error", func() {
			assert.NotNil(getErr)
		})

		s.And("The error should be of the correct type", func() {
			assert.Equal(ErrTypeNotFound, getErr)
		})
	}, t)
}

type TypeOne struct{}
type TypeTwo struct{}
