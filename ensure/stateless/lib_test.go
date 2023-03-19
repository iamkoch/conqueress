package stateful

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFullScenario(t *testing.T) {
	var (
		testCtx  = context.Background()
		aThing   bool
		tornDown = false
	)

	That("Full scenario", func(s *Scenario) {
		s.Given("a thing", func() {
			aThing = false
		})

		s.And("another thing", func() {

		})

		s.When("I do something", func() {
			aThing = true
		})

		s.Then("I should see something", func() {
			assert.Equal(t, true, aThing)
		}).Teardown("teardown", testCtx, func(ctx context.Context) {
			tornDown = true
		})
	}, t)

	assert.True(t, tornDown)
}
