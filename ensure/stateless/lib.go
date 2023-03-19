package stateful

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"testing"
)

func That(s string, f func(s *Scenario), t *testing.T) {
	fmt.Println("Scenario: " + s)
	scn := &Scenario{}
	f(scn)
	for _, f2 := range scn.teardownMethods {
		fmt.Println("Tearing down " + f2.name)
		f2.f()
	}
}

type Scenario struct {
	teardownMethods []tearDown
}

func (s2 *Scenario) Given(s string, f func()) *Scenario {
	fmt.Println(" Given " + s)
	f()
	return s2
}

func (s2 *Scenario) And(s string, f func()) *Scenario {
	fmt.Println("  And " + s)
	f()
	return s2
}

func (s2 *Scenario) When(s string, f func()) *Scenario {
	fmt.Println(" When " + s)
	f()
	return s2
}

func (s2 *Scenario) Background(s string, f func()) *Scenario {
	fmt.Println(" Background " + s)
	f()
	return s2
}

func (s2 *Scenario) Then(s string, f func()) *Scenario {
	fmt.Println(" Then " + s)
	f()
	return s2
}

// Teardown adds a function to be called when the scenario ends.
// The function is passed a context that is cancelled when the scenario ends.
func (s2 *Scenario) Teardown(s string, ctx context.Context, f func(ctx context.Context)) *Scenario {
	s2.addTearDown(s, func() {
		ctx.Done()
		f(ctx)
	})
	return s2
}

func (s2 *Scenario) addTearDown(s string, f func()) {
	s2.teardownMethods = append(s2.teardownMethods, tearDown{
		name: s,
		f:    f,
	})
}

func (s2 *Scenario) NotImplemented() {
	log.Fatal("Not implemented")
}

type tearDown struct {
	name string
	f    func()
}
