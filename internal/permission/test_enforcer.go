package permission

import (
	"github.com/casbin/casbin/v3"
)

// NewTestEnforcer creates an Enforcer with an in-memory Casbin instance
// for unit testing. Policies can be added via AddPolicy.
func NewTestEnforcer() (*Enforcer, error) {
	m := RBACModel()
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, err
	}

	return &Enforcer{
		enforcer: e,
	}, nil
}