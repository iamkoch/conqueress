package stateful

import (
	"fmt"
	"testing"
)

func ScenarioOf(s string, f func(s *Scenario), t *testing.T) {
	fmt.Print("Scenario: " + s)
	scn := &Scenario{store: make(map[string]interface{})}
	f(scn)
}

type Scenario struct {
	store           map[string]interface{}
	teardownMethods []func()
}

func (s2 *Scenario) Given(s string, f func()) {
	fmt.Println(" Given " + s)
	f()
}

func (s2 *Scenario) And(s string, f func()) {
	fmt.Println("  And " + s)
	f()
}

func (s2 *Scenario) When(s string, f func()) {
	fmt.Println(" When " + s)
	f()
}

func (s2 *Scenario) Background(s string, f func()) {
	fmt.Println(" Background " + s)
	f()
}

func (s2 *Scenario) Then(s string, f func()) {
	fmt.Println(" Then " + s)
	f()
}

func (s2 *Scenario) Teardown(s string, f func()) {
	s2.addTearDown(s, f)
}

func (s2 *Scenario) addTearDown(s string, f func()) {
	s2.teardownMethods = append(s2.teardownMethods, f)
}

func (s2 *Scenario) End() {
	for _, f := range s2.teardownMethods {
		f()
	}
}

func Store[T any](s *Scenario, key string, value T) {
	s.store[key] = value
}

func Get[T any](s *Scenario, key string) T {
	return s.store[key].(T)
}
