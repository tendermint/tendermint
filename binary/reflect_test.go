package binary

import (
	"bytes"
	"reflect"
	"testing"
)

type SimpleStruct struct {
	String string
	Bytes  []byte
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

func TestBasic(t *testing.T) {
	cat := Cat{
		SimpleStruct{
			String: "String",
			Bytes:  []byte("Bytes"),
		},
	}

	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	WriteBinary(cat, buf, n, err)
	if *err != nil {
		t.Fatalf("Failed to write cat: %v", *err)
	}
	t.Logf("Wrote bytes: %X", buf.Bytes())
	bufBytes := buf.Bytes()

	// Read onto a struct
	cat2_ := ReadBinary(Cat{}, buf, n, err)
	if *err != nil {
		t.Fatalf("Failed to read cat: %v", *err)
	}
	cat2 := cat2_.(Cat)

	if cat2.String != "String" {
		t.Errorf("Expected cat2.String == 'String', got %v", cat2.String)
	}
	if string(cat2.Bytes) != "Bytes" {
		t.Errorf("Expected cat2.Bytes == 'Bytes', got %X", cat2.Bytes)
	}

	// Read onto a ptr
	r := bytes.NewReader(bufBytes)
	cat3_ := ReadBinary(&Cat{}, r, n, err)
	if *err != nil {
		t.Fatalf("Failed to read cat: %v", *err)
	}
	cat3 := cat3_.(*Cat)

	if cat3.String != "String" {
		t.Errorf("Expected cat3.String == 'String', got %v", cat3.String)
	}
	if string(cat3.Bytes) != "Bytes" {
		t.Errorf("Expected cat3.Bytes == 'Bytes', got %X", cat3.Bytes)
	}

}

//-------------------------------------

type ComplexStruct struct {
	Name   string
	Animal Animal
}

func TestComplexStruct(t *testing.T) {
	c := ComplexStruct{
		Name: "Complex",
		Animal: Cat{
			SimpleStruct{
				String: "String",
				Bytes:  []byte("Bytes"),
			},
		},
	}

	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	WriteBinary(c, buf, n, err)
	if *err != nil {
		t.Fatalf("Failed to write c: %v", *err)
	}

	t.Logf("Wrote bytes: %X", buf.Bytes())

	c2_ := ReadBinary(&ComplexStruct{}, buf, n, err)
	if *err != nil {
		t.Fatalf("Failed to read c: %v", *err)
	}
	c2 := c2_.(*ComplexStruct)

	if cat, ok := c2.Animal.(Cat); ok {
		if cat.String != "String" {
			t.Errorf("Expected cat.String == 'String', got %v", cat.String)
		}
		if string(cat.Bytes) != "Bytes" {
			t.Errorf("Expected cat.Bytes == 'Bytes', got %X", cat.Bytes)
		}
	} else {
		t.Errorf("Expected c2.Animal to be of type cat, got %v", reflect.ValueOf(c2.Animal).Elem().Type())
	}
}

//-------------------------------------

type ComplexArrayStruct struct {
	Animals []Animal
}

func TestComplexArrayStruct(t *testing.T) {
	c := ComplexArrayStruct{
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

	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	WriteBinary(c, buf, n, err)
	if *err != nil {
		t.Fatalf("Failed to write c: %v", *err)
	}

	t.Logf("Wrote bytes: %X", buf.Bytes())

	c2_ := ReadBinary(&ComplexArrayStruct{}, buf, n, err)
	if *err != nil {
		t.Fatalf("Failed to read c: %v", *err)
	}
	c2 := c2_.(*ComplexArrayStruct)

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

type ComplexStruct2 struct {
	Cat    Cat
	Dog    *Dog
	Snake  Snake
	Snake2 *Snake
	Viper  Viper
	Viper2 *Viper
}

func TestComplexStruct2(t *testing.T) {

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

	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	WriteBinary(c, buf, n, err)
	if *err != nil {
		t.Fatalf("Failed to write c: %v", *err)
	}

	t.Logf("Wrote bytes: %X", buf.Bytes())

	c2_ := ReadBinary(&ComplexStruct2{}, buf, n, err)
	if *err != nil {
		t.Fatalf("Failed to read c: %v", *err)
	}
	c2 := c2_.(*ComplexStruct2)

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
