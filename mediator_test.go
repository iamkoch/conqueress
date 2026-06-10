package conqueress

import (
	"fmt"
	"testing"

	"github.com/iamkoch/ensure"
	"github.com/stretchr/testify/assert"
)

func TestCommandDispatch(t *testing.T) {
	var (
		mediator             *Mediator
		handler              *TestCmdHandler
		resp                 = make(chan CommandProcessingError, 1)
		commandDispatchError error
		processingError      CommandProcessingError
	)
	ensure.That("commands dispatched onto the mediator are handled", func(s *ensure.Scenario) {
		s.Given("a mediator", func() {
			mediator = NewMediator()
		})

		s.And("a command handler", func() {
			handler = &TestCmdHandler{}
			_ = RegisterCommandHandler[TestCmd](mediator, handler.Handle)
		})

		s.When("I dispatch a command and wait for the handler to complete", func() {
			commandDispatchError = mediator.Dispatch(TestCmd{
				v1: "test",
				v2: 5,
			}, resp)
			processingError = <-resp
		})

		s.Then("the submission should not error", func() {
			assert.Nil(t, commandDispatchError)
		})

		s.And("the processing should not error", func() {
			assert.Nil(t, processingError)
		})

		s.And("it should be handled", func() {
			assert.Equal(t, 1, len(handler.received))
		})

		s.And("the command should match", func() {
			cmd := handler.received[0]
			typedCmd := cmd.(TestCmd)
			assert.Equal(t, "test", typedCmd.v1)
			assert.Equal(t, 5, typedCmd.v2)
		})

	}, t)
}

func TestCommandHandlerReturnsError(t *testing.T) {
	var (
		mediator             *Mediator
		handler              *TestCmdHandler
		resp                 = make(chan CommandProcessingError, 1)
		commandDispatchError error
		processingError      CommandProcessingError
	)
	ensure.That("command handlers that return an error bubble up the error", func(s *ensure.Scenario) {
		s.Given("a mediator", func() {
			mediator = NewMediator()
		})

		s.And("a command handler that returns an error", func() {
			handler = &TestCmdHandler{}
			_ = RegisterCommandHandler[TestCmd](mediator, handler.ErrorHandle)
		})

		s.When("I dispatch a command and wait for the handler to complete", func() {
			commandDispatchError = mediator.Dispatch(TestCmd{
				v1: "test",
				v2: 5,
			}, resp)
			processingError = <-resp
		})

		s.Then("it should be handled", func() {
			assert.Equal(t, 1, len(handler.received))
		})

		s.And("the submission should not error and the processing should error", func() {
			assert.Nil(t, commandDispatchError)
			assert.NotNil(t, processingError)
		})

		s.And("the command should match", func() {
			cmd := handler.received[0]
			typedCmd := cmd.(TestCmd)
			assert.Equal(t, "test", typedCmd.v1)
			assert.Equal(t, 5, typedCmd.v2)
		})

	}, t)
}

func TestPublish(t *testing.T) {
	var (
		mediator *Mediator
		handler1 = &TestEvtHandler{}
		handler2 = &TestEvtHandler{}
	)
	ensure.That("published events are sent to all handlers", func(s *ensure.Scenario) {
		s.Given("a mediator", func() {
			mediator = NewMediator()
		})

		s.And("a handler", func() {
			_ = RegisterEventHandlers[TestEvent](mediator, handler1.Handle, handler2.Handle)
		})

		s.When("I publish an event", func() {
			_ = mediator.Publish(TestEvent{})
		})

		s.Then("it should be handled", func() {
			assert.Equal(t, 1, len(handler1.received))
			assert.Equal(t, 1, len(handler2.received))
		})

		s.And("the event should match", func() {
			evt := handler1.received[0]
			typedEvt := evt.(TestEvent)
			assert.NotNil(t, typedEvt)
		})

		s.And("the event should be the same instance", func() {
			evt1 := handler1.received[0]
			evt2 := handler2.received[0]
			assert.Equal(t, evt1, evt2)
		})

	}, t)
}

// TestEvent embeds *BaseEvent so it satisfies the full Event interface
// (MsgId, Version, WithVersion, CorrelationId, CausationId, OccurredAt,
// WithMetadata) for free. Tests construct it via NewEvent[TestEvent] so the
// embedded base is populated.
type TestEvent struct {
	*BaseEvent
}

type TestCmd struct {
	v1 string
	v2 int
}

type TestCmdHandler struct {
	received []Command
}
type TestEvtHandler struct {
	received []Event
}

func (h *TestEvtHandler) Handle(evt Event) error {
	switch evt.(type) {
	case TestEvent:
		h.received = append(h.received, evt)
	}
	return nil
}

func (t *TestCmdHandler) Handle(cmd Command) error {
	t.received = append(t.received, cmd)
	return nil
}

func (t *TestCmdHandler) ErrorHandle(cmd Command) error {
	t.received = append(t.received, cmd)
	return fmt.Errorf("some error")
}
