package guid

import (
	"github.com/pkg/errors"
	"github.com/rs/xid"
)

type Guid xid.ID

var (
	Empty = Guid(xid.NilID())
)

func New() Guid {
	return Guid(xid.New())
}

func FromString(s string) (Guid, error) {
	id, e := xid.FromString(s)
	if e != nil {
		return Empty, errors.Wrap(e, "invalid guid provided to FromString")
	}
	return Guid(id), nil
}

func MustFromString(s string) Guid {
	id, e := xid.FromString(s)
	if e != nil {
		panic("invalid guid provided to FromString")
	}
	return Guid(id)
}

func (g Guid) String() string {
	return xid.ID(g).String()
}
