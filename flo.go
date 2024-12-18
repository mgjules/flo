package flo

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"

	"github.com/dave/jennifer/jen"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/yassinebenaid/godump"
)

// Flo is a fancy name for a bidirectional graph with some opionionated rules.
// It is essentially rendered as a wrapper function declaration.
// IOs are just the function parameters and return values.
// Nodes are called components and represent function calls.
type Flo struct {
	mu             sync.Mutex
	ID             uuid.UUID
	Name           string
	Label          string
	Description    string
	PkgName        string
	PkgDescription string
	Components     map[uuid.UUID]*Component
	IOs            IOs

	// handy to quickly find a connection details.
	connectionIndex map[uuid.UUID]*ComponentConnection
}

type Component struct {
	ID          uuid.UUID
	Name        string
	PkgPath     string
	Label       string
	Description string
	Value       reflect.Value // Enable use of instantiated object's methods or functions.
	IOs         IOs
}

type ComponentIO struct {
	ID          uuid.UUID
	Name        string // autogenerated short id used as variable name.
	Type        ComponentIOType
	RType       reflect.Type
	IsError     bool
	ParentID    uuid.UUID              // Used for back reference.
	Connections []*ComponentConnection // Many outgoing but one incoming.
}

type ComponentConnection struct {
	ID               uuid.UUID
	OutComponentID   uuid.UUID
	OutComponentIOID uuid.UUID
	InComponentID    uuid.UUID
	InComponentIOID  uuid.UUID
}

type IOs []*ComponentIO

type ComponentIOType int

const (
	ComponentIOTypeUnknown ComponentIOType = iota
	ComponentIOTypeIN
	ComponentIOTypeOUT
)

// NewFlo needs fn to make IOs creation much more pleasant.
func NewFlo(
	name, label, description string,
	pkgName, pkgDescription string,
) (*Flo, error) {
	if name == "" {
		return nil, errors.New("missing name")
	}
	if label == "" {
		return nil, errors.New("missing label")
	}
	if description == "" {
		return nil, errors.New("missing description")
	}

	id := uuid.New()
	return &Flo{
		ID:              id,
		Name:            name,
		Label:           label,
		Description:     description,
		PkgName:         pkgName,
		PkgDescription:  pkgDescription,
		Components:      make(map[uuid.UUID]*Component),
		IOs:             make(IOs, 0),
		connectionIndex: make(map[uuid.UUID]*ComponentConnection),
	}, nil
}

func (f *Flo) PrettyDump(w io.Writer) error {
	var d godump.Dumper
	return d.Fprint(w, f)
}

func (f *Flo) AddIO(io *ComponentIO) error {
	if io == nil {
		return errors.New("missing io")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, found := lo.Find(f.IOs, func(fio *ComponentIO) bool {
		return fio.Name == io.Name && fio.Type == io.Type
	}); found {
		return fmt.Errorf(
			"io with same name %q and type %q already exists",
			io.Name,
			io.Type,
		)
	}

	// Ensure we have the correct parent id.
	io.ParentID = f.ID

	f.IOs = append(f.IOs, io)

	return nil
}

func (f *Flo) DeleteIO(id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.New("invalid id")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	io, found := lo.Find(f.IOs, func(io *ComponentIO) bool {
		return io.ID == id
	})
	if !found {
		return fmt.Errorf("flo io id %q not found", id)
	}

	if len(io.Connections) > 0 {
		return fmt.Errorf("flo io id %q has connections", id)
	}

	f.IOs = lo.Reject(f.IOs, func(io *ComponentIO, _ int) bool {
		return io.ID == id
	})

	return nil
}

func (f *Flo) AddComponent(c *Component) error {
	if c == nil {
		return errors.New("missing component")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, found := f.Components[c.ID]; found {
		// don't override!
		return fmt.Errorf("component id %q already exists", c.ID)
	}
	f.Components[c.ID] = c

	return nil
}

func (f *Flo) DeleteComponent(id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.New("invalid id")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if c, found := f.Components[id]; found && c.IOs.HasConnections() {
		// don't override!
		return fmt.Errorf("component id %q has connections", c.ID)
	}

	delete(f.Components, id)

	return nil
}

// ConnectComponent inter connects components or flos.
//
// Rules:
// 1. flo and component: IN(flo) -> IN(component) and OUT(component) -> OUT(flo).
// 2. component and component: OUT(component) -> IN(component).
func (f *Flo) ConnectComponent(
	outComponentID, outComponentIOID uuid.UUID,
	inComponentID, inComponentIOID uuid.UUID,
) error {
	if outComponentID == uuid.Nil {
		return errors.New("invalid out component id")
	}
	if outComponentIOID == uuid.Nil {
		return errors.New("invalid out component io id")
	}
	if inComponentID == uuid.Nil {
		return errors.New("invalid in component id")
	}
	if inComponentIOID == uuid.Nil {
		return errors.New("invalid in component io id")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	var outIOs IOs

	isFloOutgoing := outComponentID == f.ID
	if !isFloOutgoing {
		outComponent, found := f.Components[outComponentID]
		if !found {
			return fmt.Errorf("no out component id %q found in flo", outComponentID)
		}
		outIOs = outComponent.IOs
	} else {
		outIOs = f.IOs
	}
	outComponentIO, found := outIOs.GetByID(outComponentIOID)
	if !found {
		return fmt.Errorf("no component io id %q found on out component id %q", outComponentIOID, outComponentID)
	}

	var inIOs IOs

	isFloIngoing := inComponentID == f.ID
	if !isFloIngoing {
		inComponent, found := f.Components[inComponentID]
		if !found {
			return fmt.Errorf("no in component id %q found in flo", outComponentID)
		}
		inIOs = inComponent.IOs
	} else {
		inIOs = f.IOs
	}
	inComponentIO, found := inIOs.GetByID(inComponentIOID)
	if !found {
		return fmt.Errorf("no component io id %q found on in component id %q", inComponentIOID, inComponentID)
	}

	// We can't handle cyclic right now.
	if outComponentID == inComponentID {
		return fmt.Errorf("component id %q cannot connect to itself", outComponentID)
	}

	// Remember that if the component is a flo we inverse the flow check ;) (no pun intended).
	if !isFloOutgoing && outComponentIO.Type != ComponentIOTypeOUT {
		return fmt.Errorf("out component io id %q is not of type out", outComponentIOID)
	} else if isFloOutgoing && outComponentIO.Type != ComponentIOTypeIN {
		return fmt.Errorf("out flo io id %q is not of type in", outComponentIOID)
	}
	if !isFloIngoing && inComponentIO.Type != ComponentIOTypeIN {
		return fmt.Errorf("out component io id %q is not of type in", inComponentIOID)
	} else if isFloIngoing && inComponentIO.Type != ComponentIOTypeOUT {
		return fmt.Errorf("out flo io id %q is not of type out", inComponentIOID)
	}

	if len(inComponentIO.Connections) > 0 {
		return fmt.Errorf("in component io id %q already has a connection", inComponentIOID)
	}

	_, found = lo.Find(outIOs, func(io *ComponentIO) bool {
		if io == nil ||
			(!isFloOutgoing && io.Type != ComponentIOTypeOUT) ||
			(isFloOutgoing && io.Type != ComponentIOTypeIN) {
			return false
		}

		_, found := lo.Find(io.Connections, func(conn *ComponentConnection) bool {
			if conn == nil {
				return false
			}

			return conn.InComponentIOID == inComponentIO.ID
		})

		return found
	})
	if found {
		return fmt.Errorf(
			"in component id %q already has a connection with out component id %q through io id %q",
			inComponentID,
			outComponentID,
			outComponentIOID,
		)
	}

	// TODO: this might need more work than it look.
	if !outComponentIO.RType.AssignableTo(inComponentIO.RType) {
		return fmt.Errorf(
			"out component io id %q cannot be assigned to component io id %q",
			outComponentIOID,
			inComponentIOID,
		)
	}

	conn, err := NewComponentConnect(
		outComponentID, outComponentIOID,
		inComponentID, inComponentIOID,
	)
	if err != nil {
		return fmt.Errorf(
			"cannot create component connection: %v",
			err,
		)
	}

	if outComponentIO.Connections == nil {
		outComponentIO.Connections = make([]*ComponentConnection, 0)
	}
	if inComponentIO.Connections == nil {
		inComponentIO.Connections = make([]*ComponentConnection, 0)
	}

	outComponentIO.Connections = append(outComponentIO.Connections, conn)
	inComponentIO.Connections = append(inComponentIO.Connections, conn)
	f.connectionIndex[conn.ID] = conn

	inComponentIO.Name = outComponentIO.Name

	return nil
}

func (f *Flo) DeleteConnection(connectionID uuid.UUID) error {
	if connectionID == uuid.Nil {
		return errors.New("invalid connnection id")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	conn, found := f.connectionIndex[connectionID]
	if !found {
		return fmt.Errorf("unknown connection id %q", connectionID)
	}

	defer delete(f.connectionIndex, connectionID)

	outComponent, found := f.Components[conn.OutComponentID]
	if !found {
		return fmt.Errorf("no out component id %q found in flo", conn.OutComponentID)
	}
	outComponentIO, found := outComponent.IOs.GetByID(conn.OutComponentIOID)
	if !found {
		return fmt.Errorf("no component io id %q found on out component id %q", conn.OutComponentIOID, conn.OutComponentID)
	}

	outComponentIO.Connections = lo.Reject(outComponentIO.Connections, func(conn *ComponentConnection, _ int) bool {
		return conn.ID == connectionID
	})

	inComponent, found := f.Components[conn.InComponentID]
	if !found {
		return fmt.Errorf("no in component id %q found in flo", conn.OutComponentID)
	}
	inComponentIO, found := inComponent.IOs.GetByID(conn.InComponentIOID)
	if !found {
		return fmt.Errorf("no component io id %q found on in component id %q", conn.InComponentIOID, conn.InComponentID)
	}

	inComponentIO.Name = ""
	inComponentIO.Connections = make([]*ComponentConnection, 0)

	return nil
}

func (f *Flo) Render(
	ctx context.Context,
	w io.Writer,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	rendered := make(map[uuid.UUID]struct{}, len(f.Components))

	floINs, floOUTs := f.IOs.SeparateINsOUTs()

	// Generate the wrapper(flo) function.
	var blockG *jen.Group
	code := jen.NewFile(f.PkgName)
	code.HeaderComment("Code generated by flo. Do not edit!")
	code.PackageComment(f.PkgDescription)
	code.Func().Id(f.Name).
		ParamsFunc(
			func(g *jen.Group) {
				for _, in := range floINs {
					g.Do(func(s *jen.Statement) {
						if len(in.Connections) > 0 {
							s.Id(in.Name)
							return
						}
						s.Id("_")
					}).Qual(in.RType.PkgPath(), in.RType.Name())
				}
			}).
		Do(
			func(s *jen.Statement) {
				if len(floOUTs) == 0 {
					return
				}
				if len(floOUTs) == 1 {
					s.Qual(floOUTs[0].RType.PkgPath(), floOUTs[0].RType.Name())
				}
				s.Parens(jen.ListFunc(func(g *jen.Group) {
					for _, out := range floOUTs {
						g.Qual(out.RType.PkgPath(), out.RType.Name())
					}
				}))
			}).
		BlockFunc(
			func(g *jen.Group) {
				blockG = g
			},
		)

	// starts at the ingoing of a flo.
	for _, in := range floINs {
		for _, conn := range in.Connections {
			c, found := f.Components[conn.InComponentID]
			if !found {
				// Oh NO!! We should not never have a connection to a ghost component.
				return fmt.Errorf(
					"misconfigured connection id %q: missing ingoing component %q",
					conn.ID, conn.InComponentID,
				)
			}

			if err := f.RenderComponent(
				ctx,
				blockG,
				c,
				rendered,
			); err != nil {
				return fmt.Errorf(
					"failed to render component: %v", err,
				)
			}
		}
	}

	// handle orphaned components.
	for _, c := range f.Components {
		if _, found := rendered[c.ID]; found {
			continue
		}

		if err := f.RenderComponent(
			ctx,
			blockG,
			c,
			rendered,
		); err != nil {
			return fmt.Errorf(
				"failed to render component: %v", err,
			)
		}
	}

	// Generate the return statement.
	blockG.
		ReturnFunc(
			func(g *jen.Group) {
				for _, out := range floOUTs {
					if len(out.Connections) > 0 {
						g.Id(out.Name)
						continue
					}
					if out.IsError {
						g.Nil()
						continue
					}
					g.Id(fmt.Sprintf("%v", reflect.Zero(out.RType).Interface()))
				}
			},
		)

	if err := code.Render(w); err != nil {
		return err
	}

	return nil
}

func (f *Flo) RenderComponent(
	ctx context.Context,
	g *jen.Group,
	c *Component,
	rendered map[uuid.UUID]struct{},
) error {
	if c == nil {
		return errors.New("missing component")
	}
	if rendered == nil {
		return errors.New("missing rendered tracker")
	}

	if _, found := rendered[c.ID]; found {
		// Skip as we already rendered that component.
		return nil
	}

	ins, outs := c.IOs.SeparateINsOUTs()
	for _, in := range ins {
		for _, conn := range in.Connections {
			if f.ID == conn.OutComponentID {
				// Outgoing was flo so considered to have been rendered already.
				continue
			}

			if _, found := rendered[conn.OutComponentID]; found {
				continue
			}

			outC, found := f.Components[conn.OutComponentID]
			if !found {
				// Again! Ghost component!
				return fmt.Errorf(
					"misconfigured connection id %q: missing outgoing component %q",
					conn.ID, conn.OutComponentID,
				)
			}

			if err := f.RenderComponent(
				ctx,
				g,
				outC,
				rendered,
			); err != nil {
				return err
			}
		}
	}

	// Generate Go code.
	var hasErrorReturn bool
	g.
		Comment(c.Description).
		Line().
		ListFunc(func(g *jen.Group) {
			for _, out := range outs {
				if len(out.Connections) > 0 {
					g.Id(out.Name)
					continue
				}
				if out.IsError {
					hasErrorReturn = true
					g.Err()
					continue
				}
				g.Id("_")
			}
		}).
		Do(func(s *jen.Statement) {
			if len(outs) > 0 {
				s.Op(":=")
			}
		}).
		Qual(c.PkgPath, c.Name).
		CallFunc(func(g *jen.Group) {
			for _, in := range ins {
				g.Id(in.Name)
			}
		}).
		Line().
		Do(func(s *jen.Statement) {
			if hasErrorReturn {
				s.If(jen.Err().Op("!=").Nil()).Block(
					jen.ReturnFunc(func(g *jen.Group) {
						_, outs := f.IOs.SeparateINsOUTs()
						for _, out := range outs {
							if out.IsError {
								g.Err()
								continue
							}
							g.Id(fmt.Sprintf("%v", reflect.Zero(out.RType).Interface()))
						}
					}),
				).Line()
			}
		}).Line()

	rendered[c.ID] = struct{}{}

	return nil
}

func (f *Flo) Symbols() map[string]map[string]reflect.Value {
	f.mu.Lock()
	defer f.mu.Unlock()

	symbols := map[string]map[string]reflect.Value{}

	for _, c := range f.Components {
		if c.Name == "" || c.PkgPath == "" {
			continue
		}

		split := strings.Split(c.PkgPath, "/")
		pkgPath := c.PkgPath + "/" + split[len(split)-1]

		if _, found := symbols[pkgPath]; !found {
			symbols[pkgPath] = map[string]reflect.Value{}
		}

		symbols[pkgPath][c.Name] = c.Value
	}

	return symbols
}

func NewComponent(
	name, pkgPath string,
	label, description string,
	fn any,
) (*Component, error) {
	if name == "" {
		return nil, errors.New("missing name")
	}
	if pkgPath == "" {
		return nil, errors.New("missing pkg path")
	}

	c := Component{
		ID:          uuid.New(),
		Name:        name,
		PkgPath:     pkgPath,
		Label:       label,
		Description: description,
		Value:       reflect.ValueOf(fn),
	}

	if err := NewComponentIOsFromComponent(&c); err != nil {
		return nil, fmt.Errorf("cannot generate component ios: %v", err)
	}

	return &c, nil
}

func NewComponentIO(
	name string,
	typ ComponentIOType,
	rType reflect.Type,
	parentID uuid.UUID,
) (*ComponentIO, error) {
	if typ == ComponentIOTypeUnknown {
		return nil, errors.New("unknown component io type")
	}
	if name != "" {
		name = lo.CamelCase(name)
	}
	if rType == nil || rType.Kind() == reflect.Invalid {
		return nil, errors.New("invalid component io reflect type")
	}
	if parentID == uuid.Nil {
		return nil, errors.New("invalid parent ID")
	}

	return &ComponentIO{
		ID:       uuid.New(),
		Name:     name,
		Type:     typ,
		RType:    rType,
		IsError:  rType.Implements(reflect.TypeFor[error]()),
		ParentID: parentID,
	}, nil
}

func NewComponentIOsFromComponent(c *Component) error {
	if c.ID == uuid.Nil {
		return errors.New("invalid parent ID")
	}
	if !c.Value.IsValid() || c.Value.Kind() != reflect.Func {
		return fmt.Errorf("value of kind %q is not a function", c.Value.Kind())
	}

	vt := c.Value.Type()
	c.IOs = make(IOs, 0, vt.NumIn()+vt.NumOut())
	for i := 0; i < vt.NumIn(); i++ {
		p := vt.In(i)
		e, err := NewComponentIO(
			"", // Takes the name of the output io during connection.
			ComponentIOTypeIN,
			p,
			c.ID,
		)
		if err != nil {
			return fmt.Errorf("unexpected error for argument %d: %w", i+1, err)
		}

		c.IOs = append(c.IOs, e)
	}

	for i := 0; i < vt.NumOut(); i++ {
		r := vt.Out(i)
		data := sha1.Sum([]byte(fmt.Sprintf("%s-%s-%d", c.PkgPath, c.Name, i)))
		e, err := NewComponentIO(
			fmt.Sprintf("io%x", data),
			ComponentIOTypeOUT,
			r,
			c.ID,
		)
		if err != nil {
			return fmt.Errorf("unexpected error for return value %d: %w", i+1, err)
		}

		c.IOs = append(c.IOs, e)
	}

	return nil
}

func NewComponentConnect(
	outComponentID uuid.UUID,
	outComponentIOID uuid.UUID,
	inComponentID uuid.UUID,
	inComponentIOID uuid.UUID,
) (*ComponentConnection, error) {
	if outComponentID == uuid.Nil {
		return nil, errors.New("invalid out component id")
	}
	if outComponentIOID == uuid.Nil {
		return nil, errors.New("invalid out component io id")
	}
	if inComponentID == uuid.Nil {
		return nil, errors.New("invalid in component id")
	}
	if inComponentIOID == uuid.Nil {
		return nil, errors.New("invalid in component io id")
	}

	return &ComponentConnection{
		ID:               uuid.New(),
		OutComponentID:   outComponentID,
		OutComponentIOID: outComponentIOID,
		InComponentID:    inComponentID,
		InComponentIOID:  inComponentIOID,
	}, nil
}

func (ios IOs) GetByID(id uuid.UUID) (*ComponentIO, bool) {
	if ios == nil || id == uuid.Nil {
		return nil, false
	}

	return lo.Find(ios, func(io *ComponentIO) bool {
		if io == nil {
			return false
		}

		return io.ID == id
	})
}

func (ios IOs) SeparateINsOUTs() (IOs, IOs) {
	if ios == nil {
		return nil, nil
	}

	return lo.FilterReject(ios, func(io *ComponentIO, _ int) bool {
		return io.Type == ComponentIOTypeIN
	})
}

func (ios IOs) HasConnections() bool {
	if ios == nil {
		return false
	}

	return lo.SomeBy(ios, func(io *ComponentIO) bool {
		if io == nil {
			return false
		}

		return len(io.Connections) > 0
	})
}

func (t ComponentIOType) String() string {
	switch t {
	case ComponentIOTypeIN:
		return "IN"
	case ComponentIOTypeOUT:
		return "OUT"
	default:
		return "UNKNOWN"
	}
}
