package fsm

import (
	"maps"
	"slices"
)

type GuardFunc[C any] func(C) bool

type ActionFunc[C any] func(ctx C, arg string) error

type actionEntry[C any] struct {
	run     ActionFunc[C]
	allowed []string
}

type Registry[C any] struct {
	guards  map[string]GuardFunc[C]
	actions map[string]actionEntry[C]
}

func NewRegistry[C any]() *Registry[C] {
	return &Registry[C]{
		guards:  make(map[string]GuardFunc[C]),
		actions: make(map[string]actionEntry[C]),
	}
}

func (r *Registry[C]) Guard(name string, fn GuardFunc[C]) *Registry[C] {
	r.guards[name] = fn
	return r
}

func (r *Registry[C]) Action(name string, fn ActionFunc[C], allowedArgs ...string) *Registry[C] {
	r.actions[name] = actionEntry[C]{run: fn, allowed: allowedArgs}
	return r
}

func (r *Registry[C]) Effect(name string, fn func(C)) *Registry[C] {
	return r.Action(name, func(ctx C, _ string) error {
		fn(ctx)
		return nil
	}, "")
}

func (r *Registry[C]) GuardNames() []string {
	return slices.Sorted(maps.Keys(r.guards))
}

func (r *Registry[C]) ActionNames() []string {
	return slices.Sorted(maps.Keys(r.actions))
}
