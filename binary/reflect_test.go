package binary

import (
	"bytes"
	"reflect"
	"testing"
	"time"
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

func (cat Cat) TypeByte() byte { return AnimalTypeCat }

// Implements Animal
type Dog struct {
	SimpleStruct
}

func (dog Dog) TypeByte() byte { return AnimalTypeDog }

// Implements Animal
type Snake []byte

func (snake Snake) TypeByte() byte { return AnimalTypeSnake }

// Implements Animal
type Viper struct {
	Bytes []byte
}

func (viper *Viper) TypeByte() byte { return AnimalTypeViper }

var _ = RegisterInterface(
	struct{ Animal }{},
	ConcreteType{Cat{}},
	ConcreteType{Dog{}},
	ConcreteType{Snake{}},
	ConcreteType{&Viper{}},
)

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
		t.Errorf("Expected cat2.String == 'String', got %v", cat.String)
	}
	if string(cat.Bytes) != "Bytes" {
		t.Errorf("Expected cat2.Bytes == 'Bytes', got %X", cat.Bytes)
	}
	if cat.Time.Unix() != 123 {
		t.Errorf("Expected cat2.Time == 'Unix(123)', got %v", cat.Time)
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
			&Dog{ // Even though it's a *Dog, we'll get a Dog{} back.
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

//-----------------------------------------------------------------------------

var testCases = []TestCase{}

func init() {
	//testCases = append(testCases, TestCase{constructBasic, instantiateBasic, validateBasic})
	//testCases = append(testCases, TestCase{constructComplex, instantiateComplex, validateComplex})
	//testCases = append(testCases, TestCase{constructComplex2, instantiateComplex2, validateComplex2})
	testCases = append(testCases, TestCase{constructComplexArray, instantiateComplexArray, validateComplexArray})
}

func TestBinary(t *testing.T) {

	for _, testCase := range testCases {

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
			t.Fatalf("Failed to read cat: %v", *err)
		}

		// Validate object
		testCase.Validator(res, t)

		// Read onto a pointer
		n, err = new(int64), new(error)
		res = ReadBinary(instancePtr, bytes.NewReader(data), n, err)
		if *err != nil {
			t.Fatalf("Failed to read cat: %v", *err)
		}

		if res != instancePtr {
			t.Errorf("Expected pointer to pass through")
		}

		// Validate object
		testCase.Validator(reflect.ValueOf(res).Elem().Interface(), t)
	}

}

func TestJSON(t *testing.T) {

	for _, testCase := range testCases {

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
			t.Fatalf("Failed to read cat: %v", *err)
		}

		// Validate object
		testCase.Validator(res, t)

		// Read onto a pointer
		res = ReadJSON(instancePtr, data, err)
		if *err != nil {
			t.Fatalf("Failed to read cat: %v", *err)
		}

		if res != instancePtr {
			t.Errorf("Expected pointer to pass through")
		}

		// Validate object
		testCase.Validator(reflect.ValueOf(res).Elem().Interface(), t)
	}

}
