package resolve

import (
	"fmt"
	"log"
	"reflect"
)

/**
 * A Resolver is a wrapper around a ResolverFunc, which has the type signature:
 *
 *      func(in0 T0, in1 T1, in2 T2, ...) (out V, err error)
 *
 * It has one or more inputs with two outputs, the latter output being a
 * potential error. This is essentially a recipe that states:
 * "I need the input tuple (T0, T1, T2, ...) to derive an output of type V".
 *
 * Resolvers are meant to work together, e.g.:
 * resolve.Add(Resolver{func(b Boulder) Stone {...}})
 * resolve.Add(Resolver{func(t Tree) Stick {...}})
 * resolve.Add(Resolver{func(stick Stick, stone Stone) Axe {...}})
 * MyAxe, err := resolve.Resolve(Axe{},
 *                      Tree{Type: "Hickory", Height: 100 * meters},
 *                      Boulder{Weight: 1000 * kilograms},
 *               )
 */
type Resolver struct{ ResolverFunc interface{} }

func NewResolver(f interface{}) Resolver {
	r := Resolver{ResolverFunc: f}
	err := r.Validate()
	if err != nil {
		log.Fatal("NewResolver():", err)
	}
	return r
}

func (r Resolver) Validate() error {
	rType := reflect.TypeOf(r.ResolverFunc)

	if rType.Kind() != reflect.Func {
		return fmt.Errorf("Not a function")
	}

	if rType.NumIn() == 0 {
		return fmt.Errorf("No inputs")
	}

	inputTypes := map[reflect.Type]struct{}{}
	for _, v := range r.InputTypes() {
		if _, ok := inputTypes[v]; ok {
			return fmt.Errorf("Duplicate argument types not allowed")
		}
		inputTypes[v] = struct{}{}
	}

	if rType.NumOut() != 2 {
		return fmt.Errorf("Expected %d outputs, got %d", 2, rType.NumOut())
	}

	if !rType.Out(1).Implements(errorInterface) {
		return fmt.Errorf("Second output does not implement error interface")
	}

	return nil
}

func (r Resolver) Resolve(inputs ...reflect.Value) (*reflect.Value, error) {
	rType := reflect.TypeOf(r.ResolverFunc)
	rVal := reflect.ValueOf(r.ResolverFunc)

	if rType.NumIn() != len(inputs) {
		return nil, fmt.Errorf("Expected %v inputs, got %v", rType.NumIn(), len(inputs))
	}
	for i := 0; i < rType.NumIn(); i++ {
		if rType.In(i) != inputs[i].Type() {
			return nil, fmt.Errorf("Expected argument %d type to be %v, got %v",
				rType.In(i), inputs[i].Type())
		}
	}
	outputValues := rVal.Call(inputs)
	if len(outputValues) != 2 {
		return nil, fmt.Errorf("Expected %d outputs, got %d", 2, len(outputValues))
	}

	if outputValues[0].Type() != r.OutputType() {
		return nil, fmt.Errorf("Expected first output type to be %v, got %v after evaluation",
			r.OutputType(), outputValues[0].Type())
	}
	ov := outputValues[0]

	if !outputValues[1].Type().Implements(errorInterface) {
		return nil, fmt.Errorf("Second output does not implement error interface after evaluation")
	}
	var err error = nil
	if !outputValues[1].IsNil() {
		var ok bool
		err, ok = outputValues[1].Interface().(error)
		if !ok {
			return nil, fmt.Errorf("Could not type assert second output into error")
		}
	}
	return &ov, err
}

func (r Resolver) FitInputs(have []reflect.Value) (inputs []reflect.Value, err error) {
FitLoop:
	for _, wantedType := range r.InputTypes() {
		for _, value := range have {
			if value.Type() == wantedType {
				inputs = append(inputs, value)
				continue FitLoop
			}
		}
		return nil, fmt.Errorf("Could not find desired type %v in given values", wantedType)
	}
	return inputs, nil
}

func (r Resolver) MissingInputs(have []reflect.Type) (missing []reflect.Type) {
MissingLoop:
	for _, wantedType := range r.InputTypes() {
		for _, t := range have {
			if t == wantedType {
				continue MissingLoop
			}
		}
		missing = append(missing, wantedType)
	}
	return missing
}

func (r Resolver) OutputType() reflect.Type {
	rType := reflect.TypeOf(r.ResolverFunc)
	return rType.Out(0)
}

func (r Resolver) InputTypes() (types []reflect.Type) {
	rType := reflect.TypeOf(r.ResolverFunc)
	for i := 0; i < rType.NumIn(); i++ {
		types = append(types, rType.In(i))
	}
	return types
}

var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

type ProductionMap map[reflect.Type][]Resolver

func (pm ProductionMap) IsDAG() bool {
	for t := range pm {
		for i := range pm[t] {
			if pm.HasCycle(&pm[t][i], map[*Resolver]struct{}{}) {
				return false
			}
		}
	}
	return true
}

func (pm ProductionMap) HasCycle(r *Resolver, visited map[*Resolver]struct{}) bool {
	if _, ok := visited[r]; ok {
		return true
	}
	visited[r] = struct{}{}

	for _, inputType := range r.InputTypes() {
		for i := range pm[inputType] {
			visitedClone := map[*Resolver]struct{}{}
			for k, v := range visited {
				visitedClone[k] = v
			}
			if pm.HasCycle(&pm[inputType][i], visitedClone) {
				return true
			}
		}
	}
	return false
}

func (pm ProductionMap) Add(r Resolver) error {
	err := r.Validate()
	if err != nil {
		return fmt.Errorf("Failed to add Resolver: %s", err)
	}

	outputType := r.OutputType()
	for _, resolver := range pm[outputType] {
		if reflect.TypeOf(r.ResolverFunc) == reflect.TypeOf(resolver.ResolverFunc) {
			return fmt.Errorf("Resolver conflict")
		}
	}

	pm[outputType] = append(pm[outputType], r)
	if !pm.IsDAG() {
		pm[outputType] = pm[outputType][:len(pm[outputType])-1]
		if len(pm[outputType]) == 0 {
			delete(pm, outputType)
		}
		return fmt.Errorf("Circular dependencies not allowed")
	}

	return nil
}

func (pm ProductionMap) List() (resolvers []*Resolver) {
	for i := range pm {
		for j := range pm[i] {
			resolvers = append(resolvers, &pm[i][j])
		}
	}
	return resolvers
}

type PossibilityNode struct {
	*Resolver
	Inputs    []reflect.Type
	NextSteps []*PossibilityNode
}

func (pn PossibilityNode) Print(depth int) int {
	count := 1
	if pn.Resolver != nil {
		p := ""
		for i := 0; i < depth; i++ {
			p += "--"
		}
		log.Printf("%sDepth %d: Type = %v, Inputs = %v",
			p, depth, pn.OutputType(), pn.Inputs)
	}
	for _, node := range pn.NextSteps {
		count += node.Print(depth + 1)
	}
	return count
}

func (pn PossibilityNode) Count() int {
	count := 1
	for _, node := range pn.NextSteps {
		count += node.Count()
	}
	return count
}

func (pn *PossibilityNode) PruneFor(output reflect.Type) (err error) {
	if pn.Resolver != nil && pn.Resolver.OutputType() == output {
		pn.NextSteps = nil
		return nil
	}

	keepIdx := []int{}
	for i, next := range pn.NextSteps {
		err = next.PruneFor(output)
		if err != nil {
			continue
		}
		keepIdx = append(keepIdx, i)
	}
	if len(keepIdx) == 0 {
		pn.NextSteps = nil
		return fmt.Errorf("Branch could not derive %v; cutting it", output)
	}

	newNextSteps := []*PossibilityNode{}
	for _, i := range keepIdx {
		newNextSteps = append(newNextSteps, pn.NextSteps[i])
	}
	pn.NextSteps = newNextSteps
	return nil
}

func (pm ProductionMap) PossibilityTree(inputs ...reflect.Type) (root *PossibilityNode) {
	root = &PossibilityNode{}
	pm.BuildPossibilityTree(root, inputs...)
	return root
}

func (pm ProductionMap) BuildPossibilityTree(root *PossibilityNode, inputs ...reflect.Type) {
ResolverLoop:
	for _, r := range pm.List() {

		// If we already have this Resolver's output type, then we're not interested
		for _, input := range inputs {
			if r.OutputType() == input {
				continue ResolverLoop
			}
		}

		// If we're missing any inputs, then this isn't a possible next step
		mi := r.MissingInputs(inputs)
		if len(mi) > 0 {
			continue
		}

		// Clone the input types so that each PossibilityNode will have its own set
		inputsPersist := make([]reflect.Type, len(inputs))
		copy(inputsPersist, inputs)
		pn := &PossibilityNode{
			Resolver:  r,
			Inputs:    inputsPersist,
			NextSteps: []*PossibilityNode{},
		}

		inputsNext := make([]reflect.Type, len(inputs))
		copy(inputsNext, inputs)
		inputsNext = append(inputsNext, r.OutputType())
		pm.BuildPossibilityTree(pn, inputsNext...)
		root.NextSteps = append(root.NextSteps, pn)
	}
}

/*
type ResolveNode struct {
	*Resolver
	DependencyTuples [][]*ResolveNode
}

func (pm ProductionMap) BuildResolveTree(inputs ...reflect.Type) (root ResolveNode) {
}

func (pm ProductionMap) Resolve(
	wantedType reflect.Type,
	inputs ...reflect.Value,
) (
	output *reflect.Value,
	err error,
) {
	for _, input := range inputs {
		for _, resolver := range pm[input.Type()] {
			params, err := resolver.FitInputs(inputs...)
			if err != nil {
				return resolver.Resolve(params...)
			} else {
				pm.Resolve(input.Type())
			}
		}
	}
	return nil, fmt.Errorf("Could not derive requested type")
}

*/
/*
var Productions = ProductionMap{}

func Add(r Resolver) {
	err := Productions.Add(r)
	if err != nil {
		log.Fatal(err)
	}
}
*/
