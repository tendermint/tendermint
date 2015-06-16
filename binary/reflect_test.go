package binary

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
	"time"

	. "github.com/tendermint/tendermint/common"
)

type SimpleStruct struct {
	String string
	Bytes  []byte
	Time   time.Time
}

//-------------------------------------

type Animal interface{}

const (
	AnimalTypeCat   = byte(0x01)
	AnimalTypeDog   = byte(0x02)
	AnimalTypeSnake = byte(0x03)
	AnimalTypeViper = byte(0x04)
)

// Implements Animal
type Cat struct {
	SimpleStruct
}

// Implements Animal
type Dog struct {
	SimpleStruct
}

// Implements Animal
type Snake []byte

// Implements Animal
type Viper struct {
	Bytes []byte
}

var _ = RegisterInterface(
	struct{ Animal }{},
	ConcreteType{Cat{}, AnimalTypeCat},
	ConcreteType{Dog{}, AnimalTypeDog},
	ConcreteType{Snake{}, AnimalTypeSnake},
	ConcreteType{&Viper{}, AnimalTypeViper},
)

func TestAnimalInterface(t *testing.T) {
	var foo Animal

	// Type of pointer to Animal
	rt := reflect.TypeOf(&foo)
	fmt.Printf("rt: %v\n", rt)

	// Type of Animal itself.
	// NOTE: normally this is acquired through other means
	// like introspecting on method signatures, or struct fields.
	rte := rt.Elem()
	fmt.Printf("rte: %v\n", rte)

	// Get a new pointer to the interface
	// NOTE: calling .Interface() is to get the actual value,
	// instead of reflection values.
	ptr := reflect.New(rte).Interface()
	fmt.Printf("ptr: %v", ptr)

	// Make a binary byteslice that represents a snake.
	snakeBytes := BinaryBytes(Snake([]byte("snake")))
	snakeReader := bytes.NewReader(snakeBytes)

	// Now you can read it.
	n, err := new(int64), new(error)
	it := *ReadBinary(ptr, snakeReader, n, err).(*Animal)
	fmt.Println(it, reflect.TypeOf(it))
}

//-------------------------------------

type Constructor func() interface{}
type Instantiator func() (o interface{}, ptr interface{})
type Validator func(o interface{}, t *testing.T)

type TestCase struct {
	Constructor
	Instantiator
	Validator
}

//-------------------------------------

func constructBasic() interface{} {
	cat := Cat{
		SimpleStruct{
			String: "String",
			Bytes:  []byte("Bytes"),
			Time:   time.Unix(123, 0),
		},
	}
	return cat
}

func instantiateBasic() (interface{}, interface{}) {
	return Cat{}, &Cat{}
}

func validateBasic(o interface{}, t *testing.T) {
	cat := o.(Cat)
	if cat.String != "String" {
		t.Errorf("Expected cat.String == 'String', got %v", cat.String)
	}
	if string(cat.Bytes) != "Bytes" {
		t.Errorf("Expected cat.Bytes == 'Bytes', got %X", cat.Bytes)
	}
	if cat.Time.Unix() != 123 {
		t.Errorf("Expected cat.Time == 'Unix(123)', got %v", cat.Time)
	}
}

//-------------------------------------

type NilTestStruct struct {
	IntPtr *int
	CatPtr *Cat
	Animal Animal
}

func constructNilTestStruct() interface{} {
	return NilTestStruct{}
}

func instantiateNilTestStruct() (interface{}, interface{}) {
	return NilTestStruct{}, &NilTestStruct{}
}

func validateNilTestStruct(o interface{}, t *testing.T) {
	nts := o.(NilTestStruct)
	if nts.IntPtr != nil {
		t.Errorf("Expected nts.IntPtr to be nil, got %v", nts.IntPtr)
	}
	if nts.CatPtr != nil {
		t.Errorf("Expected nts.CatPtr to be nil, got %v", nts.CatPtr)
	}
	if nts.Animal != nil {
		t.Errorf("Expected nts.Animal to be nil, got %v", nts.Animal)
	}
}

//-------------------------------------

type ComplexStruct struct {
	Name   string
	Animal Animal
}

func constructComplex() interface{} {
	c := ComplexStruct{
		Name:   "Complex",
		Animal: constructBasic(),
	}
	return c
}

func instantiateComplex() (interface{}, interface{}) {
	return ComplexStruct{}, &ComplexStruct{}
}

func validateComplex(o interface{}, t *testing.T) {
	c2 := o.(ComplexStruct)
	if cat, ok := c2.Animal.(Cat); ok {
		validateBasic(cat, t)
	} else {
		t.Errorf("Expected c2.Animal to be of type cat, got %v", reflect.ValueOf(c2.Animal).Elem().Type())
	}
}

//-------------------------------------

type ComplexStruct2 struct {
	Cat    Cat
	Dog    *Dog
	Snake  Snake
	Snake2 *Snake
	Viper  Viper
	Viper2 *Viper
}

func constructComplex2() interface{} {
	snake_ := Snake([]byte("hiss"))
	snakePtr_ := &snake_

	c := ComplexStruct2{
		Cat: Cat{
			SimpleStruct{
				String: "String",
				Bytes:  []byte("Bytes"),
			},
		},
		Dog: &Dog{
			SimpleStruct{
				String: "Woof",
				Bytes:  []byte("Bark"),
			},
		},
		Snake:  Snake([]byte("hiss")),
		Snake2: snakePtr_,
		Viper:  Viper{Bytes: []byte("hizz")},
		Viper2: &Viper{Bytes: []byte("hizz")},
	}
	return c
}

func instantiateComplex2() (interface{}, interface{}) {
	return ComplexStruct2{}, &ComplexStruct2{}
}

func validateComplex2(o interface{}, t *testing.T) {
	c2 := o.(ComplexStruct2)
	cat := c2.Cat
	if cat.String != "String" {
		t.Errorf("Expected cat.String == 'String', got %v", cat.String)
	}
	if string(cat.Bytes) != "Bytes" {
		t.Errorf("Expected cat.Bytes == 'Bytes', got %X", cat.Bytes)
	}

	dog := c2.Dog
	if dog.String != "Woof" {
		t.Errorf("Expected dog.String == 'Woof', got %v", dog.String)
	}
	if string(dog.Bytes) != "Bark" {
		t.Errorf("Expected dog.Bytes == 'Bark', got %X", dog.Bytes)
	}

	snake := c2.Snake
	if string(snake) != "hiss" {
		t.Errorf("Expected string(snake) == 'hiss', got %v", string(snake))
	}

	snake2 := c2.Snake2
	if string(*snake2) != "hiss" {
		t.Errorf("Expected string(snake2) == 'hiss', got %v", string(*snake2))
	}

	viper := c2.Viper
	if string(viper.Bytes) != "hizz" {
		t.Errorf("Expected string(viper.Bytes) == 'hizz', got %v", string(viper.Bytes))
	}

	viper2 := c2.Viper2
	if string(viper2.Bytes) != "hizz" {
		t.Errorf("Expected string(viper2.Bytes) == 'hizz', got %v", string(viper2.Bytes))
	}
}

//-------------------------------------

type ComplexStructArray struct {
	Animals []Animal
}

func constructComplexArray() interface{} {
	c := ComplexStructArray{
		Animals: []Animal{
			Cat{
				SimpleStruct{
					String: "String",
					Bytes:  []byte("Bytes"),
				},
			},
			Dog{
				SimpleStruct{
					String: "Woof",
					Bytes:  []byte("Bark"),
				},
			},
			Snake([]byte("hiss")),
			&Viper{
				Bytes: []byte("hizz"),
			},
		},
	}
	return c
}

func instantiateComplexArray() (interface{}, interface{}) {
	return ComplexStructArray{}, &ComplexStructArray{}
}

func validateComplexArray(o interface{}, t *testing.T) {
	c2 := o.(ComplexStructArray)
	if cat, ok := c2.Animals[0].(Cat); ok {
		if cat.String != "String" {
			t.Errorf("Expected cat.String == 'String', got %v", cat.String)
		}
		if string(cat.Bytes) != "Bytes" {
			t.Errorf("Expected cat.Bytes == 'Bytes', got %X", cat.Bytes)
		}
	} else {
		t.Errorf("Expected c2.Animals[0] to be of type cat, got %v", reflect.ValueOf(c2.Animals[0]).Elem().Type())
	}

	if dog, ok := c2.Animals[1].(Dog); ok {
		if dog.String != "Woof" {
			t.Errorf("Expected dog.String == 'Woof', got %v", dog.String)
		}
		if string(dog.Bytes) != "Bark" {
			t.Errorf("Expected dog.Bytes == 'Bark', got %X", dog.Bytes)
		}
	} else {
		t.Errorf("Expected c2.Animals[1] to be of type dog, got %v", reflect.ValueOf(c2.Animals[1]).Elem().Type())
	}

	if snake, ok := c2.Animals[2].(Snake); ok {
		if string(snake) != "hiss" {
			t.Errorf("Expected string(snake) == 'hiss', got %v", string(snake))
		}
	} else {
		t.Errorf("Expected c2.Animals[2] to be of type Snake, got %v", reflect.ValueOf(c2.Animals[2]).Elem().Type())
	}

	if viper, ok := c2.Animals[3].(*Viper); ok {
		if string(viper.Bytes) != "hizz" {
			t.Errorf("Expected string(viper.Bytes) == 'hizz', got %v", string(viper.Bytes))
		}
	} else {
		t.Errorf("Expected c2.Animals[3] to be of type *Viper, got %v", reflect.ValueOf(c2.Animals[3]).Elem().Type())
	}
}

//-------------------------------------

type MapStruct struct {
	String string
	Map1   map[string]int
	Map2   map[string]string
}

func constructMap() interface{} {
	mapper := MapStruct{
		String: "String",
		Map1:   map[string]int{"hi": 5, "bye": 6, "howdy": 7},
		Map2:   map[string]string{"hi": "5", "bye": "6", "howdy": "7"},
	}
	return mapper
}

func instantiateMap() (interface{}, interface{}) {
	return MapStruct{}, &MapStruct{}
}

func validateMap(o interface{}, t *testing.T) {
	mapper := o.(MapStruct)
	if mapper.String != "String" {
		t.Errorf("Expected mapper.String == 'String', got %v", mapper.String)
	}
	if mapper.Map1["hi"] != 5 {
		t.Errorf("Expected Map1['hi']== 5, got %d", mapper.Map1["hi"])
	}
	if mapper.Map1["bye"] != 6 {
		t.Errorf("Expected Map1['bye']== 6, got %d", mapper.Map1["bye"])
	}
	if mapper.Map1["howdy"] != 7 {
		t.Errorf("Expected Map1['howdy']== 7, got %d", mapper.Map1["howdy"])
	}

	if mapper.Map2["hi"] != "5" {
		t.Errorf("Expected Map2['hi']== '5', got %s", mapper.Map2["hi"])
	}
	if mapper.Map2["bye"] != "6" {
		t.Errorf("Expected Map2['bye']== '6', got %s", mapper.Map2["bye"])
	}
	if mapper.Map2["howdy"] != "7" {
		t.Errorf("Expected Map2['howdy']== '7', got %s", mapper.Map2["howdy"])
	}
}

//-------------------------------------

var testCases = []TestCase{}

func init() {
	testCases = append(testCases, TestCase{constructBasic, instantiateBasic, validateBasic})
	testCases = append(testCases, TestCase{constructComplex, instantiateComplex, validateComplex})
	testCases = append(testCases, TestCase{constructComplex2, instantiateComplex2, validateComplex2})
	testCases = append(testCases, TestCase{constructComplexArray, instantiateComplexArray, validateComplexArray})
	testCases = append(testCases, TestCase{constructNilTestStruct, instantiateNilTestStruct, validateNilTestStruct})
}

func TestBinary(t *testing.T) {

	for i, testCase := range testCases {

		log.Info(fmt.Sprintf("Running test case %v", i))

		// Construct an object
		o := testCase.Constructor()

		// Write the object
		data := BinaryBytes(o)
		t.Logf("Binary: %X", data)

		instance, instancePtr := testCase.Instantiator()

		// Read onto a struct
		n, err := new(int64), new(error)
		res := ReadBinary(instance, bytes.NewReader(data), n, err)
		if *err != nil {
			t.Fatalf("Failed to read into instance: %v", *err)
		}

		// Validate object
		testCase.Validator(res, t)

		// Read onto a pointer
		n, err = new(int64), new(error)
		res = ReadBinary(instancePtr, bytes.NewReader(data), n, err)
		if *err != nil {
			t.Fatalf("Failed to read into instance: %v", *err)
		}

		if res != instancePtr {
			t.Errorf("Expected pointer to pass through")
		}

		// Validate object
		testCase.Validator(reflect.ValueOf(res).Elem().Interface(), t)
	}

}

func TestJSON(t *testing.T) {

	testCasesWithMap := append(testCases, TestCase{constructMap, instantiateMap, validateMap})
	for i, testCase := range testCasesWithMap {

		log.Info(fmt.Sprintf("Running test case %v", i))

		// Construct an object
		o := testCase.Constructor()

		// Write the object
		data := JSONBytes(o)
		t.Logf("JSON: %v", string(data))

		instance, instancePtr := testCase.Instantiator()

		// Read onto a struct
		err := new(error)
		res := ReadJSON(instance, data, err)
		if *err != nil {
			t.Fatalf("Failed to read object onto struct: %v", *err)
		}

		// Validate object
		testCase.Validator(res, t)

		// Read onto a pointer
		res = ReadJSON(instancePtr, data, err)
		if *err != nil {
			t.Fatalf("Failed to read object onto pointer: %v", *err)
		}

		if res != instancePtr {
			t.Errorf("Expected pointer to pass through")
		}

		// Validate object
		testCase.Validator(reflect.ValueOf(res).Elem().Interface(), t)
	}

}

//------------------------------------------------------------------------------

type Foo struct {
	FieldA string `json:"fieldA"` // json field name is "fieldA"
	FieldB string // json field name is "FieldB"
	fieldC string // not exported, not serialized.
}

func TestJSONFieldNames(t *testing.T) {
	for i := 0; i < 20; i++ { // Try to ensure deterministic success.
		foo := Foo{"a", "b", "c"}
		stringified := string(JSONBytes(foo))
		expected := `{"fieldA":"a","FieldB":"b"}`
		if stringified != expected {
			t.Fatalf("JSONFieldNames error: expected %v, got %v",
				expected, stringified)
		}
	}
}

//------------------------------------------------------------------------------

func TestBadAlloc(t *testing.T) {
	n, err := new(int64), new(error)
	instance := new([]byte)
	data := RandBytes(ByteSliceChunk * 100)
	b := new(bytes.Buffer)
	// this slice of data claims to be much bigger than it really is
	WriteUvarint(uint(10000000000000000), b, n, err)
	b.Write(data)
	res := ReadBinary(instance, b, n, err)
	fmt.Println(res, *err)
}
