package conqueress

import (
	"errors"
	"log/slog"
	"reflect"
	"sync"
)

type CommandHandler func(cmd Command) error
type EventProcessor func(evt Event) error

// Mediator dispatches commands to their registered handlers and publishes
// events to their registered processors.
//
// Commands are dispatched on a background goroutine (Dispatch) or synchronously
// (DispatchSync); events publish to N processors concurrently (Publish) or
// sequentially (PublishSync).
//
// The core is intentionally deterministic. If you want artificial delays in
// tests to exercise eventual-consistency behaviour, wrap a handler with a
// delay decorator at registration time — don't bake it into the framework.
type Mediator struct {
	commandQueue    chan queuedCommand
	commandHandlers map[reflect.Type]CommandHandler
	eventProcessors map[reflect.Type][]EventProcessor
}

type queuedCommand struct {
	cmd                 Command
	synchronousResponse chan CommandProcessingError
}

// NewMediator constructs an empty mediator and starts its background
// command-processing goroutine.
func NewMediator() *Mediator {
	mediator := &Mediator{
		commandQueue:    make(chan queuedCommand),
		commandHandlers: make(map[reflect.Type]CommandHandler),
		eventProcessors: make(map[reflect.Type][]EventProcessor),
	}

	go mediator.processCommands()
	return mediator
}

func (m *Mediator) processCommands() {
	for cmdReq := range m.commandQueue {
		cmd := cmdReq.cmd
		slog.With(
			"command", cmd,
			"type", reflect.TypeOf(cmd),
		).Debug("Processing command")
		resp := cmdReq.synchronousResponse
		handler := m.commandHandlers[reflect.TypeOf(cmd)]

		result := handler(cmd)
		slog.With(
			"command", cmd,
			"type", reflect.TypeOf(cmd),
			"result", result,
		).Debug("Command processed")
		if resp != nil {
			resp <- result
		}
	}
}

func RegisterCommandHandler[T Command](m *Mediator, handler CommandHandler) error {
	var t T
	return m.RegisterCommandHandler(reflect.TypeOf(t), handler)
}

func RegisterEventHandlers[T Event](m *Mediator, handlers ...EventProcessor) error {
	var t T
	eventType := reflect.TypeOf(t)
	errors := make([]error, 0)
	for _, h := range handlers {
		e := m.RegisterEventHandler(eventType, h)
		if e != nil {
			errors = append(errors, e)
		}
	}
	if len(errors) > 0 {
		return &EventRegistrationError{
			EventType: eventType,
			Errors:    errors,
		}
	}

	return nil
}

func (m *Mediator) WaitForCommands() {
	close(m.commandQueue)
}

type EventRegistrationError struct {
	EventType reflect.Type
	Errors    []error
}

func (e *EventRegistrationError) Error() string {
	return "error registering event handlers"
}

func (m *Mediator) RegisterCommandHandler(cmdType reflect.Type, handler CommandHandler) error {
	if _, exists := m.commandHandlers[cmdType]; exists {
		return errors.New("command handler already registered")
	}
	slog.With(
		"command", cmdType,
	).Info("Registering command handler")
	m.commandHandlers[cmdType] = handler
	return nil
}

func (m *Mediator) RegisterEventHandler(evtType reflect.Type, handler EventProcessor) error {
	handlers, existing := m.eventProcessors[evtType]
	if !existing {
		handlers = make([]EventProcessor, 0)
	}
	for _, p := range handlers {
		if reflect.DeepEqual(p, handler) {
			return errors.New("processor already registered")
		}
	}
	handlers = append(handlers, handler)
	m.eventProcessors[evtType] = handlers
	return nil
}

func (m *Mediator) Dispatch(cmd Command, syncResp chan CommandProcessingError) CommandSubmissionError {
	of := reflect.TypeOf(cmd)
	if _, ok := m.commandHandlers[of]; ok {
		slog.With(
			"type", of,
			"command", cmd,
		).Info("Dispatching command")
		m.commandQueue <- queuedCommand{cmd, syncResp}
		return nil
	}
	return errors.New("no handler registered")
}

func (m *Mediator) DispatchSync(cmd Command, syncResp chan CommandProcessingError) CommandSubmissionError {
	of := reflect.TypeOf(cmd)
	if _, ok := m.commandHandlers[of]; ok {
		slog.With(
			"type", of,
			"command", cmd,
		).Info("Dispatching command")
		slog.With(
			"command", cmd,
			"type", reflect.TypeOf(cmd),
		).Debug("Processing command")
		handler := m.commandHandlers[reflect.TypeOf(cmd)]

		result := handler(cmd)
		slog.With(
			"command", cmd,
			"type", reflect.TypeOf(cmd),
			"result", result,
		).Debug("Command processed")
		return result
	}
	return errors.New("no handler registered")
}

// Publish fans the event out to all registered processors concurrently and
// blocks until every processor has returned. Errors from individual
// processors are not surfaced to the caller; instrument inside each
// processor if you need observability.
//
// Blocking on completion is deliberate. Fire-and-forget publish makes
// downstream side effects untestable and hides handler errors. If you want
// non-blocking semantics, wrap the call in a goroutine at the call site.
func (m *Mediator) Publish(evt Event) error {
	processors, ok := m.eventProcessors[reflect.TypeOf(evt)]
	if !ok {
		return errors.New("no processor registered")
	}

	var wg sync.WaitGroup
	wg.Add(len(processors))
	for _, processor := range processors {
		go func(p EventProcessor) {
			defer wg.Done()
			_ = p(evt)
		}(processor)
	}
	wg.Wait()
	return nil
}

// PublishSync runs each processor sequentially on the caller's goroutine.
// Errors from individual processors are not surfaced (consistent with
// Publish); wrap a processor with error-aware middleware if needed.
func (m *Mediator) PublishSync(evt Event) error {
	if processors, ok := m.eventProcessors[reflect.TypeOf(evt)]; ok {
		for _, processor := range processors {
			_ = processor(evt)
		}
		return nil
	}
	return errors.New("no processor registered")
}

type CommandProcessingError error
type CommandSubmissionError error

type CommandDispatcher interface {
	Dispatch(e Command, synchronousResponse chan CommandProcessingError) CommandSubmissionError
	DispatchSync(e Command) CommandSubmissionError
}

type EventPublisher interface {
	Publish(e Event) error
	PublishSync(e Event) error
}
