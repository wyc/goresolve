package resolve

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

type Manager struct {
	ID      int64
	CanHire bool
	CanFire bool
}

type Employee struct {
	ID        int64
	ManagerID int64
	Title     string
	Salary    float64
}

type User struct {
	ID           int64
	Email        string
	PasswordHash string
}

// User if the request has a valid user session, nil and error otherwise
func RequestToUser(w http.ResponseWriter, r *http.Request) (*User, error) {
	return &User{Email: "person@company.com", PasswordHash: "0xdeadbeef"}, nil
}

// Manager role if the User has one, nil and error otherwise
func (u User) GetManager() (*Manager, error) {
	return &Manager{CanHire: true, CanFire: false}, nil
}

// Employee role if the User has one, nil and error otherwise
func (u User) GetEmployee() (*Employee, error) {
	return &Employee{Title: "Paper Pusher", Salary: 100000.00}, nil
}

func ManagerInfo(w http.ResponseWriter, r *http.Request, m Manager) (err error) {
	_, err = w.Write([]byte(fmt.Sprintf("Can Hire: %v\nCan Fire: %v", m.CanHire, m.CanFire)))
	return err
}

func EmployeeInfo(w http.ResponseWriter, r *http.Request, e Employee) (err error) {
	_, err = w.Write([]byte(fmt.Sprintf("Title: %v\nSalary: $%0.2f", e.Title, e.Salary)))
	return err
}

func UserInfo(w http.ResponseWriter, r *http.Request, u User) (err error) {
	_, err = w.Write([]byte(fmt.Sprintf("Email: %v", u.Email)))
	return err
}

// User if the request has a valid user session, nil and error otherwise
func StringToUser(id ID, email Email, passwordHash PasswordHash) (*User, error) {
	return &User{ID: int64(id), Email: string(email), PasswordHash: string(passwordHash)}, nil
}

type ID int64
type Email string
type PasswordHash string

var TestUserID = ID(1337)
var TestUserEmail = Email("fred@example.com")
var TestUserPasswordHash = PasswordHash("0xdeadbeef")

func TestResolveUser(t *testing.T) {
	r := NewResolver(StringToUser)

	userValue, err := r.Resolve(
		reflect.ValueOf(TestUserID),
		reflect.ValueOf(TestUserEmail),
		reflect.ValueOf(TestUserPasswordHash),
	)
	if err != nil {
		t.Fatal(err)
	}
	user, ok := userValue.Interface().(*User)
	if !ok {
		t.Fatal("Returned type was not *User")
	}

	switch {
	case ID(user.ID) != TestUserID:
		t.Error("ID mismatch")
	case Email(user.Email) != TestUserEmail:
		t.Error("Email mismatch")
	case PasswordHash(user.PasswordHash) != TestUserPasswordHash:
		t.Error("Password mismatch")
	}
}

func TestResolveMismatchedInputTypes(t *testing.T) {
	wronglyTypedPasswordHash := 4444 // Should be string
	r := NewResolver(StringToUser)
	_, err := r.Resolve(
		reflect.ValueOf(TestUserID),
		reflect.ValueOf(TestUserEmail),
		reflect.ValueOf(wronglyTypedPasswordHash),
	)
	if err == nil {
		t.Fatal("No error incurred")
	}
}

func TestResolveTooFewInputs(t *testing.T) {
	r := NewResolver(StringToUser)
	_, err := r.Resolve(
		reflect.ValueOf(TestUserID),
	)
	if err == nil {
		t.Fatal("No error incurred")
	}
}

func TestResolveTooManyInputs(t *testing.T) {
	r := NewResolver(StringToUser)
	_, err := r.Resolve(
		reflect.ValueOf(TestUserID),
		reflect.ValueOf(TestUserEmail),
		reflect.ValueOf(TestUserPasswordHash),
		reflect.ValueOf(TestUserID),
	)
	if err == nil {
		t.Fatal("No error incurred")
	}
}

func TestValidateResolvers(t *testing.T) {
	var err error
	goodResolver :=
		Resolver{func(email string) (*User, error) {
			return &User{}, nil
		}}
	err = goodResolver.Validate()
	if err != nil {
		t.Fatal("goodResolver did not validate")
	}

	badResolverNoErrorOutput :=
		Resolver{func(email string) (*User, string) {
			return &User{}, ""
		}}
	err = badResolverNoErrorOutput.Validate()
	if err == nil {
		t.Fatal("badResolverNoErrorOutput validated")
	}

	badResolverTooFewOutputs :=
		Resolver{func(email string) *User {
			return &User{}
		}}
	err = badResolverTooFewOutputs.Validate()
	if err == nil {
		t.Fatal("badResolverTooFewOutputs validated")
	}

	badResolverTooManyOutputs :=
		Resolver{func(email string) (*User, error, string) {
			return &User{}, nil, ""
		}}
	err = badResolverTooManyOutputs.Validate()
	if err == nil {
		t.Fatal("badResolverTooManyOutputs validated")
	}

	badResolverNoInputs :=
		Resolver{func() (*User, error) {
			return &User{}, nil
		}}
	err = badResolverNoInputs.Validate()
	if err == nil {
		t.Fatal("badResolverNoInputs validated")
	}
}

func TestProductionMapDuplicate(t *testing.T) {
	productions := ProductionMap{}

	f := func(name string) (*User, error) {
		return &User{}, nil
	}

	err := productions.Add(Resolver{f})
	if err != nil {
		t.Fatal("Failed to add first Resolver")
	}
	err = productions.Add(Resolver{f})
	if err == nil {
		t.Fatal("Duplicate production added with no error")
	}
}

func TestProductionMapOneNodeCycle(t *testing.T) {
	productions := ProductionMap{}

	f := func(user *User) (*User, error) {
		return &User{}, nil
	}

	err := productions.Add(Resolver{f})
	if err == nil {
		t.Fatal("Cyclic production added with no error")
	}

	if len(productions) > 0 {
		t.Fatal("Failed to remove invalid Resolver")
	}
}

func TestProductionMapTwoNodeCycle(t *testing.T) {
	productions := ProductionMap{}

	g := func(i int) (string, error) {
		return "", nil
	}

	h := func(s string) (int, error) {
		return 0, nil
	}

	err := productions.Add(Resolver{g})
	if err != nil {
		t.Fatal("Failed to add first valid Resolver")
	}
	err = productions.Add(Resolver{h})
	if err == nil {
		t.Fatal("Cyclic production chain not reported")
	}
}

func TestProductionMapThreeNodeCycle(t *testing.T) {
	productions := ProductionMap{}

	g := func(i int) (string, error) {
		return "", nil
	}

	h := func(s string) (float64, error) {
		return 0.0, nil
	}

	i := func(f float64) (int, error) {
		return 0, nil
	}

	err := productions.Add(Resolver{g})
	if err != nil {
		t.Fatal("Failed to add first valid Resolver")
	}
	err = productions.Add(Resolver{h})
	if err != nil {
		t.Fatal("Failed to add second valid Resolver")
	}
	err = productions.Add(Resolver{i})
	if err == nil {
		t.Fatal("Cyclic production chain not reported")
	}
}

func TestProductionMapThreeNodeNoCycle(t *testing.T) {
	productions := ProductionMap{}

	g := func(i int) (string, error) {
		return "", nil
	}

	h := func(s string) (float64, error) {
		return 0.0, nil
	}

	i := func(f float64) (byte, error) {
		return 0, nil
	}

	err := productions.Add(Resolver{g})
	if err != nil {
		t.Fatal("Failed to add first valid Resolver")
	}
	err = productions.Add(Resolver{h})
	if err != nil {
		t.Fatal("Failed to add second valid Resolver")
	}
	err = productions.Add(Resolver{i})
	if err != nil {
		t.Fatal("Failed to add third valid Resolver")
	}
}

type Boulder struct{ Weight int }
type Tree struct{ Height int }
type Stick struct{ Height int }
type Stone struct{ Weight int }
type Axe struct{ Weight, Height int }

func TestPossibilityTreeBuildAxe(t *testing.T) {
	productions := ProductionMap{}

	chiselStone := func(b Boulder) (Stone, error) { return Stone{Weight: b.Weight / 10}, nil }
	pickupStick := func(t Tree) (Stick, error) { return Stick{Height: t.Height / 10}, nil }
	assembleAxe := func(stick Stick, stone Stone) (Axe, error) {
		return Axe{Height: stick.Height, Weight: stone.Weight}, nil
	}

	var err error

	err = productions.Add(Resolver{chiselStone})
	if err != nil {
		t.Fatal("Failed to add chisel Resolver:", err)
	}
	pn := productions.PossibilityTree(
		reflect.TypeOf(Boulder{}),
		reflect.TypeOf(Tree{}),
	)
	// [Boulder Tree]
	// [Boulder Tree Stone]
	if pn.Count() != 2 {
		t.Fatalf("Invalid Possibility count %d, expected %d", pn.Count(), 1)
	}

	err = productions.Add(Resolver{pickupStick})
	if err != nil {
		t.Fatal("Failed to add pickup Resolver:", err)
	}

	pn = productions.PossibilityTree(
		reflect.TypeOf(Boulder{}),
		reflect.TypeOf(Tree{}),
	)
	// [Boulder Tree]
	// -- [Boulder Tree Stone]
	// ---- [Boulder Tree Stone Stick]
	// -- [Boulder Tree Stick]
	// ---- [Boulder Tree Stick Stone]
	if pn.Count() != 5 {
		t.Fatalf("Invalid possibility count %d, expected %d", pn.Count(), 5)
	}

	err = productions.Add(Resolver{assembleAxe})
	if err != nil {
		t.Fatal("Failed to add assemble Resolver:", err)
	}

	pn = productions.PossibilityTree(
		reflect.TypeOf(Boulder{}),
		reflect.TypeOf(Tree{}),
	)
	// [Boulder Tree]
	// -- [Boulder Tree Stone]
	// ---- [Boulder Tree Stone Stick]
	// ------ [Boulder Tree Stone Stick Axe]
	// -- [Boulder Tree Stick]
	// ---- [Boulder Tree Stick Stone]
	// ------ [Boulder Tree Stick Stone Axe]
	if pn.Count() != 7 {
		t.Fatalf("Invalid possibility count %d, expected %d", pn.Count(), 7)
	}

	type Hammer struct{ Weight, Height int }

	assembleHammer := func(stone Stone, stick Stick) (Hammer, error) {
		return Hammer{Weight: stone.Weight, Height: stick.Height}, nil
	}
	err = productions.Add(Resolver{assembleHammer})
	if err != nil {
		t.Fatal("Failed to add assembleHammer Resolver", err)
	}
	pn = productions.PossibilityTree(
		reflect.TypeOf(Boulder{}),
		reflect.TypeOf(Tree{}),
	)
	// [Boulder Tree]
	// -- [Boulder Tree Stone]
	// ---- [Boulder Tree Stone Stick]
	// ------ [Boulder Tree Stone Stick Axe]
	// -------- [Boulder Tree Stone Stick Axe Hammer]
	// ------ [Boulder Tree Stone Stick Hammer]
	// -------- [Boulder Tree Stone Stick Hammer Axe]
	// -- [Boulder Tree Stick]
	// ---- [Boulder Tree Stick Stone]
	// ------ [Boulder Tree Stick Stone Axe]
	// -------- [Boulder Tree Stick Stone Axe Hammer]
	// ------ [Boulder Tree Stick Stone Hammer]
	// -------- [Boulder Tree Stick Stone Hammer Axe]
	if pn.Count() != 13 {
		t.Fatalf("Invalid possibility count %d, expected %d", pn.Count(), 13)
	}

}

// AddIdentityResolverHTTP(User{}, RequestToUser)

// AddRoleResolver(User.GetManager)
// AddRoleResolver(User.GetEmployee)

// http.Handle("/manager", AuthedHTTP(ManagerInfo))
// http.Handle("/employee", AuthedHTTP(EmployeeInfo))
// http.Handle("/user", AuthedHTTP(UserInfo))

/*
	http.Handle("/manager", managerHandler(ManagerInfo))
	http.Handle("/employee", employeeHandler(EmployeeInfo))
	http.Handle("/user", userHandler(UserInfo))
	log.Println("Listening on :8000")
	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
*/
