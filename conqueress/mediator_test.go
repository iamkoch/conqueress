package conqueress

import (
	"fmt"
	"github.com/iamkoch/conqueress/guid"
	"github.com/iamkoch/ensure"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestCommandDispatch(t *testing.T) {
	var (
		mediator             *Mediator
		handler              *TestCmdHandler
		resp                 = make(chan CommandProcessingError)
		commandDispatchError error
	)
	ensure.That("published commands are dispatched without delay when induce is false", func(s *ensure.Scenario) {
		s.Given("a command dispatcher with induce delay false", func() {
			mediator = NewMediator(false)
		})

		s.And("a command handler", func() {
			handler = &TestCmdHandler{}
			_ = RegisterCommandHandler[TestCmd](mediator, handler.Handle)
		})

		s.When("I publish a command", func() {
			commandDispatchError = mediator.Dispatch(TestCmd{
				v1: "test",
				v2: 5,
			}, resp)
			time.Sleep(10 * time.Millisecond)
		})

		s.Then("it should not error", func() {
			assert.Nil(t, commandDispatchError)
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
		resp                 = make(chan CommandProcessingError)
		commandDispatchError error
	)
	ensure.That("command handlers that return an error bubble up the error", func(s *ensure.Scenario) {
		s.Given("a command dispatcher with induce delay false", func() {
			mediator = NewMediator(false)
		})

		s.And("a command handler", func() {
			handler = &TestCmdHandler{}
			_ = RegisterCommandHandler[TestCmd](mediator, handler.ErrorHandle)
		})

		s.When("I publish a command", func() {
			commandDispatchError = mediator.Dispatch(TestCmd{
				v1: "test",
				v2: 5,
			}, resp)
			time.Sleep(500 * time.Millisecond)
		})

		s.Then("it should be handled", func() {
			assert.Equal(t, 1, len(handler.received))
		})

		s.And("it should error", func() {
			var caughtErr error
			select {
			case caughtErr = <-resp:
			}
			assert.Nil(t, commandDispatchError)
			assert.NotNil(t, caughtErr)
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
			mediator = NewMediator(false)
		})

		s.And("a handler", func() {
			_ = RegisterEventHandlers[TestEvent](mediator, handler1.Handle, handler2.Handle)
		})

		s.When("I publish an event", func() {
			mediator.Publish(TestEvent{})
			time.Sleep(100 * time.Millisecond)
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

type TestEvent struct {
}

func (t TestEvent) MsgId() guid.Guid {
	//TODO implement me
	panic("implement me")
}

func (t TestEvent) WithVersion(v int) {
	//TODO implement me
	panic("implement me")
}

func (t TestEvent) Version() int {
	panic("implement me")
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
