goresolve
=========

goresolve is combinator library for Go. Not working yet.
Fill it up with recipes called `Resolvers` like this:

```go
type Boulder struct{ Weight int }
type Tree struct{ Height int }
type Stick struct{ Height int }
type Stone struct{ Weight int }
type Axe struct{ Weight, Height int }

func ChiselStone(b Boulder) (*Stone, error) {
        return &Stone{Weight: b.Weight / 10}, nil
}

func StealBranch(t Tree) (*Stick, error) {
        return &Stick{Height: t.Height / 10}, nil
}

func AssembleAxe(stick Stick, stone Stone) (*Axe, error) {
        return &Axe{Height: stick.Height, Weight: stone.Weight}, nil
}

/**
 * Resolvers have a type signature of:
 *         func(in0 T0, in1 T1, in2 T2, ...) (out *V, err error) { ... }
 * The inputs are the required resources and output is what is produced.
 *
 * Resolvers MUST:
 * - Have a distinct type per input argument
 * - Have different types among other resolvers
 * - Have no circular dependencies to each other (e.g. NOT A->B->A or A->A)
 */
func main() {
        resolve.Add(Resolver{ChiselStone})
        resolve.Add(Resolver{StealBranch})
        resolve.Add(Resolver{AssembleAxe})

        // Try to craft an Axe given a Tree and Boulder
        maybeAxe, err := resolve.Resolve(Axe{}, Tree{1000}, Boulder{500})
        if err != nil {
            log.Fatal(err)
        }
        axe := maybeAxe.(Axe)
        axe.Chop()
}
```

