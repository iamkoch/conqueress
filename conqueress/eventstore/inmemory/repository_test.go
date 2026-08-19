package inmemory

import (
	"reflect"
	"testing"

	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/domain"
	"github.com/iamkoch/conqueress/eventstore"
	"github.com/iamkoch/conqueress/guid"
	. "github.com/smartystreets/goconvey/convey"
)

type User struct {
	domain.AggregateRootBase[guid.Guid]
	name string
}

func (u *User) SetBase(base domain.AggregateRootBase[guid.Guid]) {
	u.AggregateRootBase = base
}

func (u *User) GetHandler() func(e cqrs.Event) {
	return u.handleEvent
}

func (u *User) SetInnerApply(ia func(e cqrs.Event)) {
	u.AggregateRootBase.SetInnerApply(ia)
}

type UserCreated struct {
	*cqrs.BaseEvent
	id   guid.Guid
	name string
}

func (u *User) handleEvent(e cqrs.Event) {
	switch evt := e.(type) {
	case UserCreated:
		u.SetId(evt.id)
		u.SetVersion(evt.Ver)
		u.name = evt.name
	}
}

func NewUser2() *User {
	return domain.New[User]()
}

func NewUser() *User {
	u := User{
		AggregateRootBase: domain.NewAggregate[guid.Guid](),
	}
	u.SetInnerApply(u.handleEvent)
	return &u
}

func TestRepository(t *testing.T) {
	Convey("save and load generics works", t, func() {
		var cap cqrs.Event
		h := func(e cqrs.Event) error {
			cap = e
			return nil
		}
		m := cqrs.NewMediator(false)
		storage := NewInMemoryEventStore[guid.Guid](m)
		repo := eventstore.NewRepository[*User](storage, domain.GetDefaultAggregate[User])
		m.RegisterEventHandler(reflect.TypeOf(UserCreated{}), h)

		agg := NewUser2()
		id := guid.New()
		agg.ApplyChange(UserCreated{&cqrs.BaseEvent{
			MessageId: guid.New().String(),
			Ver:       -1,
		}, id, "bob"})

		repo.Save(agg, -1)

		loaded, err := repo.GetById(id)
		So(err, ShouldBeNil)
		So(loaded.name, ShouldEqual, "bob")
		So(loaded.Id(), ShouldEqual, id)

		So(cap, ShouldNotBeNil)
	})
}
