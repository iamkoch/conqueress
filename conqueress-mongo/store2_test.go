package store

//
//import (
//	"fmt"
//	cqrs "github.com/iamkoch/conqueress"
//	"github.com/iamkoch/conqueress/eventstore"
//	"github.com/iamkoch/conqueress/guid"
//	. "github.com/onsi/ginkgo/v2"
//	. "github.com/onsi/gomega"
//	"reflect"
//	"sample_domain"
//	"time"
//)
//
//type testPublisher struct {
//	capturedEvents []cqrs.Event
//}
//
//func newTestPublisher() *testPublisher {
//	return &testPublisher{capturedEvents: make([]cqrs.Event, 0)}
//}
//
//func (t *testPublisher) Publish(event cqrs.Event) {
//	t.capturedEvents = append(t.capturedEvents, event)
//}
//
//func (t *testPublisher) Handle(event cqrs.Event) error {
//	t.capturedEvents = append(t.capturedEvents, event)
//	return nil
//}
//
//type MongoFixture struct {
//	mediator *cqrs.Mediator
//	repo     eventstore.Repository[*sample_domain.InventoryItem]
//}
//
//func NewFixture(id guid.Guid) *MongoFixture {
//	waitForSuccess := make(chan cqrs.CommandProcessingError)
//	var repo eventstore.Repository[*sample_domain.InventoryItem]
//	var m *cqrs.Mediator
//
//	tm := NewTypeMap().Add(sample_domain.InventoryItemCreated{}).Add(sample_domain.InventoryItemRenamed{})
//	m = cqrs.NewMediator(false)
//	s, err := NewMongoEventStore("mongodb://admin-user:admin-password@mongodb:27017", tm)
//	if err != nil {
//		panic(err.Error())
//	}
//	repo = eventstore.NewRepository[*sample_domain.InventoryItem](s, sample_domain.DefaultInventoryItem)
//	commands := sample_domain.NewInventoryCommandHandler(repo)
//	m.RegisterCommandHandler(reflect.TypeOf(sample_domain.CreateInventoryItem{}), commands.HandleCreateInventoryItem)
//	m.RegisterCommandHandler(reflect.TypeOf(sample_domain.RenameInventoryItem{}), commands.HandleRenameInventoryItem)
//	handler := newTestPublisher()
//	m.RegisterEventHandler(reflect.TypeOf(sample_domain.InventoryItemCreated{}), handler.Handle)
//	time.Sleep(time.Second)
//	return &MongoFixture{mediator: m, repo: repo}
//}
//
//var _ = Describe("Conqueress Mongo Store", func() {
//
//	Describe("Saving and loading an aggregate", func() {
//
//	})
//
//	Describe("save and load", func() {
//
//		Describe("when saving and loading", func() {
//
//			err := m.Dispatch(sample_domain.NewCreateInventoryItem(actualId, "something"), waitForSuccess)
//			if err != nil {
//				fmt.Errorf("something went wrong dispatching %s", err.Error())
//			}
//			err = <-waitForSuccess
//			if err != nil {
//				fmt.Errorf("something went processing %s", err.Error())
//			}
//			ii := repo.GetById(actualId)
//
//			It("should have the correct name", func() {
//				Expect(ii.Name).To(Equal("something"))
//			})
//
//		})
//
//		Describe("when saving and loading again", func() {
//			var waitForSuccess chan cqrs.CommandProcessingError = nil
//			var repo eventstore.Repository[*sample_domain.InventoryItem]
//			var m *cqrs.Mediator
//			var actualId guid.Guid
//			BeforeEach(func() {
//				actualId = guid.New()
//
//				waitForSuccess = make(chan cqrs.CommandProcessingError)
//				tm := NewTypeMap().Add(sample_domain.InventoryItemCreated{}).Add(sample_domain.InventoryItemRenamed{})
//				m = cqrs.NewMediator(false)
//				s, err := NewMongoEventStore("mongodb://admin-user:admin-password@mongodb:27017", tm)
//				if err != nil {
//					panic(err.Error())
//				}
//				repo = eventstore.NewRepository[*sample_domain.InventoryItem](s, sample_domain.DefaultInventoryItem)
//				commands := sample_domain.NewInventoryCommandHandler(repo)
//				m.RegisterCommandHandler(reflect.TypeOf(sample_domain.CreateInventoryItem{}), commands.HandleCreateInventoryItem)
//				m.RegisterCommandHandler(reflect.TypeOf(sample_domain.RenameInventoryItem{}), commands.HandleRenameInventoryItem)
//				handler := newTestPublisher()
//				m.RegisterEventHandler(reflect.TypeOf(sample_domain.InventoryItemCreated{}), handler.Handle)
//				time.Sleep(time.Second)
//			})
//			err := m.Dispatch(sample_domain.NewRenameInventoryItem(actualId, "something new"), waitForSuccess)
//			if err != nil {
//				fmt.Errorf("something went wrong dispatching %s", err.Error())
//			}
//			err = <-waitForSuccess
//			if err != nil {
//				fmt.Errorf("something went processing %s", err.Error())
//			}
//
//			ii := repo.GetById(actualId)
//			It("should have the correct name", func() {
//				Expect(ii.Name).To(Equal("something new"))
//			})
//		})
//	})
//
//	Describe("concurrency", func() {
//		Describe("Given a configured test domain", func() {
//			var repo eventstore.Repository[*sample_domain.InventoryItem]
//			var m *cqrs.Mediator
//			var actualId guid.Guid
//			BeforeEach(func() {
//				actualId = guid.New()
//
//				tm := NewTypeMap().Add(sample_domain.InventoryItemCreated{}).Add(sample_domain.InventoryItemRenamed{})
//				m = cqrs.NewMediator(false)
//				s, err := NewMongoEventStore("mongodb://admin-user:admin-password@mongodb:27017", tm)
//				if err != nil {
//					panic(err.Error())
//				}
//				repo = eventstore.NewRepository[*sample_domain.InventoryItem](s, sample_domain.DefaultInventoryItem)
//				commands := sample_domain.NewInventoryCommandHandler(repo)
//				m.RegisterCommandHandler(reflect.TypeOf(sample_domain.CreateInventoryItem{}), commands.HandleCreateInventoryItem)
//				m.RegisterCommandHandler(reflect.TypeOf(sample_domain.RenameInventoryItem{}), commands.HandleRenameInventoryItem)
//				handler := newTestPublisher()
//				m.RegisterEventHandler(reflect.TypeOf(sample_domain.InventoryItemCreated{}), handler.Handle)
//				time.Sleep(time.Second)
//			})
//			m = cqrs.NewMediator(false)
//			tm := NewTypeMap().Add(sample_domain.InventoryItemCreated{}).Add(sample_domain.InventoryItemRenamed{})
//
//			s, err := NewMongoEventStore("mongodb://admin-user:admin-password@mongodb:27017", tm)
//			if err != nil {
//				panic(err.Error())
//			}
//			repo = eventstore.NewRepository[*sample_domain.InventoryItem](s, sample_domain.DefaultInventoryItem)
//			commands := sample_domain.NewInventoryCommandHandler(repo)
//			m.RegisterCommandHandler(reflect.TypeOf(sample_domain.CreateInventoryItem{}), commands.HandleCreateInventoryItem)
//			m.RegisterCommandHandler(reflect.TypeOf(sample_domain.RenameInventoryItem{}), commands.HandleRenameInventoryItem)
//
//			handler := newTestPublisher()
//			m.RegisterEventHandler(reflect.TypeOf(sample_domain.InventoryItemCreated{}), handler.Handle)
//			time.Sleep(time.Second)
//			actualId = guid.New()
//			err = m.Dispatch(sample_domain.NewCreateInventoryItem(actualId, "something"), nil)
//			if err != nil {
//				fmt.Errorf("something went wrong dispatching %s", err.Error())
//			}
//			time.Sleep(time.Second * 2)
//
//			ii := repo.GetById(actualId)
//			It("should have the correct name", func() {
//				Expect(ii.Name()).To(Equal("something"))
//			})
//
//			err = m.Dispatch(sample_domain.NewRenameInventoryItem(actualId, "something new"), nil)
//			err = m.Dispatch(sample_domain.NewRenameInventoryItem(actualId, "something new 2"), nil)
//			err = m.Dispatch(sample_domain.NewRenameInventoryItem(actualId, "something new 3"), nil)
//			if err != nil {
//				fmt.Errorf("something went wrong dispatching %s", err.Error())
//			}
//			time.Sleep(time.Second * 2)
//
//			ii = repo.GetById(actualId)
//			It("should have the correct name", func() {
//				Expect(ii.Name()).To(Equal("something new 3"))
//			})
//		})
//	})
//})
