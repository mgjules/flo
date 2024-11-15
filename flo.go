package flo

import (
	"reflect"

	"github.com/google/uuid"
)

type Component struct {
	ID          uuid.UUID
	Label       string
	Description string
	Value       reflect.Value // Enable use of instantiated object.
	Type        ComponentType
	INs         []ComponentEdge
	OUTs        []ComponentEdge
}

type ComponentEdge struct {
	Label       string
	Description string
	Type        reflect.Type
	Connection  *ComponentConnection
}

type ComponentConnection struct {
	ID        string // autogenerated short ID (possibly seeded using the Component, of "Flo" type, ID).
	Component *Component
}

type ComponentType int

const (
	ComponentTypeUnknown ComponentType = iota

	// Acts like a flo descriptor (does not do any actual work).
	// Component Value is always empty.
	ComponentTypeFlo

	// A normal node in a flo that represent a unit of work.
	ComponentTypeNode
)