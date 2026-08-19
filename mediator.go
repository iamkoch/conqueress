package conqueress

import (
	"errors"
	"log/slog"
	"math/rand"
	"reflect"
	"time"
)

type CommandHandler func(cmd Command) error
type EventProcessor func(evt Event) error

type Command interface {
}

type Mediator struct {
	commandQueue    chan queuedCommand
	commandHandlers map[reflect.Type]CommandHandler
	eventProcessors map[reflect.Type][]EventProcessor
	induceDelay     bool
}

type queuedCommand struct {
	cmd                 Command
	synchronousResponse chan CommandProcessingError
}

func NewMediator(induceDelay bool) *Mediator {
	mediator := &Mediator{
		commandQueue:    make(chan queuedCommand),
		commandHandlers: make(map[reflect.Type]CommandHandler),
		eventProcessors: make(map[reflect.Type][]EventProcessor),
		induceDelay:     induceDelay,
	}

	go mediator.processCommands()
	return mediator
}

func (m *Mediator) processCommands() {
	for {
		select {
		case cmdReq := <-m.commandQueue:
			cmd := cmdReq.cmd
			slog.With(
				"command", cmd,
				"type", reflect.TypeOf(cmd),
			).Debug("Processing command")
			resp := cmdReq.synchronousResponse
			handler, _ := m.commandHandlers[reflect.TypeOf(cmd)]
			if m.induceDelay {
				time.Sleep(time.Duration(1*rand.Intn(3)) * time.Second)
			}

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
		handler, _ := m.commandHandlers[reflect.TypeOf(cmd)]
		if m.induceDelay {
			time.Sleep(time.Duration(1*rand.Intn(3)) * time.Second)
		}

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

func (m *Mediator) Publish(evt Event) error {
	if processors, ok := m.eventProcessors[reflect.TypeOf(evt)]; ok {
		for _, processor := range processors {
			go func(p EventProcessor) {
				if m.induceDelay {
					time.Sleep(time.Duration(rand.Intn(10)) * time.Second) // Have a variable degree of eventual consistency
				}
				p(evt)
			}(processor)
		}
		return nil
	}
	return errors.New("no processor registered")
}

func (m *Mediator) PublishSync(evt Event) error {
	if processors, ok := m.eventProcessors[reflect.TypeOf(evt)]; ok {
		for _, processor := range processors {
			func(p EventProcessor) {
				if m.induceDelay {
					time.Sleep(time.Duration(rand.Intn(10)) * time.Second) // Have a variable degree of eventual consistency
				}
				p(evt)
			}(processor)
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
