package wire

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	cmn "github.com/tendermint/tmlibs/common"
)

type SimpleStruct struct {
	String string
	Bytes  []byte
	Time   time.Time
}

type Animal interface{}

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
	ConcreteType{Cat{}, 0x01},
	ConcreteType{Dog{}, 0x02},
	ConcreteType{Snake{}, 0x03},
	ConcreteType{&Viper{}, 0x04},
)

func TestTime(t *testing.T) {

	// panic trying to encode times before 1970
	panicCases := []time.Time{
		time.Time{},
		time.Unix(-10, 0),
		time.Unix(0, -10),
	}
	for _, c := range panicCases {
		n, err := new(int), new(error)
		buf := new(bytes.Buffer)
		assert.Panics(t, func() { WriteBinary(c, buf, n, err) }, "expected WriteBinary to panic on times before 1970")
	}

	// ensure we can encode/decode a recent time
	now := time.Now()
	n, err := new(int), new(error)
	buf := new(bytes.Buffer)
	WriteBinary(now, buf, n, err)

	var thisTime time.Time
	thisTime = ReadBinary(thisTime, buf, 0, new(int), new(error)).(time.Time)
	if !thisTime.Truncate(time.Millisecond).Equal(now.Truncate(time.Millisecond)) {
		t.Fatalf("times dont match. got %v, expected %v", thisTime, now)
	}

	// error trying to decode bad times
	errorCases := []struct {
		thisTime time.Time
		err      error
	}{
		{time.Time{}, ErrBinaryReadInvalidTimeNegative},
		{time.Unix(-10, 0), ErrBinaryReadInvalidTimeNegative},
		{time.Unix(0, -10), ErrBinaryReadInvalidTimeNegative},

		{time.Unix(0, 10), ErrBinaryReadInvalidTimeSubMillisecond},
		{time.Unix(1, 10), ErrBinaryReadInvalidTimeSubMillisecond},
	}
	for _, c := range errorCases {
		n, err := new(int), new(error)
		buf := new(bytes.Buffer)
		timeNano := c.thisTime.UnixNano()
		WriteInt64(timeNano, buf, n, err)
		var thisTime time.Time
		thisTime = ReadBinary(thisTime, buf, 0, n, err).(time.Time)
		assert.Equal(t, *err, c.err, "expected ReadBinary to throw an error")
		assert.Equal(t, thisTime, time.Time{}, "expected ReadBinary to return default time")
	}
}

func TestEncodeDecode(t *testing.T) {
	cat := &Cat{SimpleStruct{String: "cat", Time: time.Now()}}

	n, err := new(int), new(error)
	buf := new(bytes.Buffer)
	WriteBinary(cat, buf, n, err)
	if *err != nil {
		t.Fatalf("writeBinary:: failed to encode Cat: %v", *err)
	}

	cat2 := new(Cat)
	n, err = new(int), new(error)
	cat2 = ReadBinary(cat2, buf, 0, n, err).(*Cat)
	if *err != nil {
		t.Fatalf("unexpected err: %v", *err)
	}

	// NOTE: this fails because []byte{} != []byte(nil)
	// 	assert.Equal(t, cat, cat2, "expected cats to match")
}

func TestUnexportedEmbeddedTypes(t *testing.T) {
	type unexportedReceiver struct {
		animal Animal
	}

	type exportedReceiver struct {
		Animal Animal
	}

	now := time.Now().Truncate(time.Millisecond)
	origCat := Cat{SimpleStruct{String: "cat", Time: now}}
	exportedCat := exportedReceiver{origCat} // this is what we encode
	writeCat := func() *bytes.Buffer {
		n, err := new(int), new(error)
		buf := new(bytes.Buffer)
		WriteBinary(exportedCat, buf, n, err)
		if *err != nil {
			t.Errorf("writeBinary:: failed to encode Cat: %v", *err)
		}
		return buf
	}

	// try to read into unexportedReceiver (should fail)
	buf := writeCat()
	n, err := new(int), new(error)
	unexp := ReadBinary(unexportedReceiver{}, buf, 0, n, err).(unexportedReceiver)
	if *err != nil {
		t.Fatalf("unexpected err: %v", *err)
	}
	returnCat, ok := unexp.animal.(Cat)
	if ok {
		t.Fatalf("unexpectedly parsed out the Cat type")
	}

	// try to read into exportedReceiver (should pass)
	buf = writeCat()
	n, err = new(int), new(error)
	exp := ReadBinary(exportedReceiver{}, buf, 0, n, err).(exportedReceiver)
	if *err != nil {
		t.Fatalf("unexpected err: %v", *err)
	}
	returnCat, ok = exp.Animal.(Cat)
	if !ok {
		t.Fatalf("expected to be able to parse out the Cat type; rrecv: %#v", exp.Animal)
	}

	_ = returnCat
	// NOTE: this fails because []byte{} != []byte(nil)
	//	assert.Equal(t, origCat, returnCat, fmt.Sprintf("cats dont match"))

}

// TODO: add assertions here ...
func TestAnimalInterface(t *testing.T) {
	var foo Animal

	// Type of pointer to Animal
	rt := reflect.TypeOf(&foo)

	// Type of Animal itself.
	// NOTE: normally this is acquired through other means
	// like introspecting on method signatures, or struct fields.
	rte := rt.Elem()

	// Get a new pointer to the interface
	// NOTE: calling .Interface() is to get the actual value,
	// instead of reflection values.
	reflect.New(rte).Interface()

	// Make a binary byteslice that represents a *snake.
	foo = Snake([]byte("snake"))
	snakeBytes := BinaryBytes(foo)
	snakeReader := bytes.NewReader(snakeBytes)

	// Now you can read it.
	n, err := new(int), new(error)
	animal := ReadBinary(foo, snakeReader, 0, n, err).(Animal)
	assert.NotNil(t, animal)
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
			Time:   time.Unix(123, 456789999),
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
	if cat.Time.UnixNano() != 123456000000 { // Only milliseconds
		t.Errorf("Expected cat.Time.UnixNano() == 123456000000, got %v", cat.Time.UnixNano())
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
				Time:   time.Now(),
			},
		},
		Dog: &Dog{
			SimpleStruct{
				String: "Woof",
				Bytes:  []byte("Bark"),
				Time:   time.Now(),
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
	Bytes   [5]byte
	Ints    [5]int
	Array   SimpleArray
}

func constructComplexArray() interface{} {
	c := ComplexStructArray{
		Animals: []Animal{
			Cat{
				SimpleStruct{
					String: "String",
					Bytes:  []byte("Bytes"),
					Time:   time.Now(),
				},
			},
			Dog{
				SimpleStruct{
					String: "Woof",
					Bytes:  []byte("Bark"),
					Time:   time.Now(),
				},
			},
			Snake([]byte("hiss")),
			&Viper{
				Bytes: []byte("hizz"),
			},
		},
		Bytes: [5]byte{1, 10, 50, 100, 200},
		Ints:  [5]int{1, 2, 3, 4, 5},
		Array: SimpleArray([5]byte{1, 10, 50, 100, 200}),
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
	testCases = append(testCases, TestCase{constructBasic, instantiateBasic, validateBasic})
	testCases = append(testCases, TestCase{constructComplex, instantiateComplex, validateComplex})
	testCases = append(testCases, TestCase{constructComplex2, instantiateComplex2, validateComplex2})
	testCases = append(testCases, TestCase{constructComplexArray, instantiateComplexArray, validateComplexArray})
	testCases = append(testCases, TestCase{constructNilTestStruct, instantiateNilTestStruct, validateNilTestStruct})
}

func TestBinary(t *testing.T) {

	for i, testCase := range testCases {

		t.Log(fmt.Sprintf("Running test case %v", i))

		// Construct an object
		o := testCase.Constructor()

		// Write the object
		data := BinaryBytes(o)
		t.Logf("Binary: %X", data)

		instance, instancePtr := testCase.Instantiator()

		// Read onto a struct
		n, err := new(int), new(error)
		res := ReadBinary(instance, bytes.NewReader(data), 0, n, err)
		if *err != nil {
			t.Fatalf("Failed to read into instance: %v", *err)
		}

		// Validate object
		testCase.Validator(res, t)

		// Read onto a pointer
		n, err = new(int), new(error)
		res = ReadBinaryPtr(instancePtr, bytes.NewReader(data), 0, n, err)
		if *err != nil {
			t.Fatalf("Failed to read into instance: %v", *err)
		}
		if res != instancePtr {
			t.Errorf("Expected pointer to pass through")
		}

		// Validate object
		testCase.Validator(reflect.ValueOf(res).Elem().Interface(), t)

		// Read with len(data)-1 limit should fail.
		instance, _ = testCase.Instantiator()
		n, err = new(int), new(error)
		ReadBinary(instance, bytes.NewReader(data), len(data)-1, n, err)
		if *err != ErrBinaryReadOverflow {
			t.Fatalf("Expected ErrBinaryReadOverflow")
		}

		// Read with len(data) limit should succeed.
		instance, _ = testCase.Instantiator()
		n, err = new(int), new(error)
		ReadBinary(instance, bytes.NewReader(data), len(data), n, err)
		if *err != nil {
			t.Fatalf("Failed to read instance with sufficient limit: %v n: %v len(data): %v type: %v",
				(*err).Error(), *n, len(data), reflect.TypeOf(instance))
		}
	}

}

func TestJSON(t *testing.T) {

	for i, testCase := range testCases {

		t.Log(fmt.Sprintf("Running test case %v", i))

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

//------------------------------------------------------------------------------

type Foo struct {
	FieldA string `json:"fieldA"` // json field name is "fieldA"
	FieldB string // json field name is "FieldB"
	fieldC string // not exported, not serialized.
	FieldD string `json:",omitempty"`  // omit if empty
	FieldE string `json:",omitempty"`  // omit if empty (but won't be)
	FieldF string `json:"F,omitempty"` // its name is "F", omit if empty
	FieldG string `json:"G,omitempty"` // its name is "F", omit if empty (but won't be)
}

func TestJSONFieldNames(t *testing.T) {
	for i := 0; i < 20; i++ { // Try to ensure deterministic success.
		foo := Foo{
			FieldA: "a",
			FieldB: "b",
			fieldC: "c",
			FieldD: "",  // omit because empty
			FieldE: "e", // no omit, not empty
			FieldF: "",  // omit because empty
			FieldG: "g", // no omit, not empty
		}
		stringified := string(JSONBytes(foo))
		expected := `{"fieldA":"a","FieldB":"b","FieldE":"e","G":"g"}`
		if stringified != expected {
			t.Fatalf("JSONFieldNames error: expected %v, got %v",
				expected, stringified)
		}
	}
}

//------------------------------------------------------------------------------

func TestBadAlloc(t *testing.T) {
	n, err := new(int), new(error)
	instance := new([]byte)
	data := cmn.RandBytes(100 * 1024)
	b := new(bytes.Buffer)
	// this slice of data claims to be much bigger than it really is
	WriteUvarint(uint(1<<32-1), b, n, err)
	b.Write(data)
	ReadBinary(instance, b, 0, n, err)
}

//------------------------------------------------------------------------------

type SimpleArray [5]byte

func TestSimpleArray(t *testing.T) {
	var foo SimpleArray

	// Type of pointer to array
	rt := reflect.TypeOf(&foo)

	// Type of array itself.
	// NOTE: normally this is acquired through other means
	// like introspecting on method signatures, or struct fields.
	rte := rt.Elem()

	// Get a new pointer to the array
	// NOTE: calling .Interface() is to get the actual value,
	// instead of reflection values.
	reflect.New(rte).Interface()

	// Make a simple int aray
	fooArray := SimpleArray([5]byte{1, 10, 50, 100, 200})
	fooBytes := BinaryBytes(fooArray)
	fooReader := bytes.NewReader(fooBytes)

	// Now you can read it.
	n, err := new(int), new(error)
	it := ReadBinary(foo, fooReader, 0, n, err).(SimpleArray)

	if !bytes.Equal(it[:], fooArray[:]) {
		t.Errorf("Expected %v but got %v", fooArray, it)
	}
}

//--------------------------------------------------------------------------------

func TestNilPointerInterface(t *testing.T) {

	type MyInterface interface{}
	type MyConcreteStruct1 struct{}
	type MyConcreteStruct2 struct{}

	RegisterInterface(
		struct{ MyInterface }{},
		ConcreteType{&MyConcreteStruct1{}, 0x01},
		ConcreteType{&MyConcreteStruct2{}, 0x02},
	)

	type MyStruct struct {
		MyInterface
	}

	myStruct := MyStruct{(*MyConcreteStruct1)(nil)}
	buf, n, err := new(bytes.Buffer), int(0), error(nil)
	WriteBinary(myStruct, buf, &n, &err)
	if err == nil {
		t.Error("Expected error in writing nil pointer interface")
	}

	myStruct = MyStruct{&MyConcreteStruct1{}}
	buf, n, err = new(bytes.Buffer), int(0), error(nil)
	WriteBinary(myStruct, buf, &n, &err)
	if err != nil {
		t.Error("Unexpected error", err)
	}

}

//--------------------------------------------------------------------------------

func TestMultipleInterfaces(t *testing.T) {

	type MyInterface1 interface{}
	type MyInterface2 interface{}
	type Struct1 struct{}
	type Struct2 struct{}

	RegisterInterface(
		struct{ MyInterface1 }{},
		ConcreteType{&Struct1{}, 0x01},
		ConcreteType{&Struct2{}, 0x02},
		ConcreteType{Struct1{}, 0x03},
		ConcreteType{Struct2{}, 0x04},
	)
	RegisterInterface(
		struct{ MyInterface2 }{},
		ConcreteType{&Struct1{}, 0x11},
		ConcreteType{&Struct2{}, 0x12},
		ConcreteType{Struct1{}, 0x13},
		ConcreteType{Struct2{}, 0x14},
	)

	type MyStruct struct {
		F1 []MyInterface1
		F2 []MyInterface2
	}

	myStruct := MyStruct{
		F1: []MyInterface1{
			nil,
			&Struct1{},
			&Struct2{},
			Struct1{},
			Struct2{},
		},
		F2: []MyInterface2{
			nil,
			&Struct1{},
			&Struct2{},
			Struct1{},
			Struct2{},
		},
	}
	buf, n, err := new(bytes.Buffer), int(0), error(nil)
	WriteBinary(myStruct, buf, &n, &err)
	if err != nil {
		t.Error("Unexpected error", err)
	}
	if hexStr := hex.EncodeToString(buf.Bytes()); hexStr !=
		"0105"+"0001020304"+"0105"+"0011121314" {
		t.Error("Unexpected binary bytes", hexStr)
	}

	// Now, read

	myStruct2 := MyStruct{}
	ReadBinaryPtr(&myStruct2, buf, 0, &n, &err)
	if err != nil {
		t.Error("Unexpected error", err)
	}

	if len(myStruct2.F1) != 5 {
		t.Error("Expected F1 to have 5 items")
	}
	if myStruct2.F1[0] != nil {
		t.Error("Expected F1[0] to be nil")
	}
	if _, ok := (myStruct2.F1[1]).(*Struct1); !ok {
		t.Error("Expected F1[1] to be of type *Struct1")
	}
	if s, _ := (myStruct2.F1[1]).(*Struct1); s == nil {
		t.Error("Expected F1[1] to be of type *Struct1 but not nil")
	}
	if _, ok := (myStruct2.F1[2]).(*Struct2); !ok {
		t.Error("Expected F1[2] to be of type *Struct2")
	}
	if s, _ := (myStruct2.F1[2]).(*Struct2); s == nil {
		t.Error("Expected F1[2] to be of type *Struct2 but not nil")
	}
	if _, ok := (myStruct2.F1[3]).(Struct1); !ok {
		t.Error("Expected F1[3] to be of type Struct1")
	}
	if _, ok := (myStruct2.F1[4]).(Struct2); !ok {
		t.Error("Expected F1[4] to be of type Struct2")
	}
	if myStruct2.F2[0] != nil {
		t.Error("Expected F2[0] to be nil")
	}
	if _, ok := (myStruct2.F2[1]).(*Struct1); !ok {
		t.Error("Expected F2[1] to be of type *Struct1")
	}
	if s, _ := (myStruct2.F2[1]).(*Struct1); s == nil {
		t.Error("Expected F2[1] to be of type *Struct1 but not nil")
	}
	if _, ok := (myStruct2.F2[2]).(*Struct2); !ok {
		t.Error("Expected F2[2] to be of type *Struct2")
	}
	if s, _ := (myStruct2.F2[2]).(*Struct2); s == nil {
		t.Error("Expected F2[2] to be of type *Struct2 but not nil")
	}
	if _, ok := (myStruct2.F2[3]).(Struct1); !ok {
		t.Error("Expected F2[3] to be of type Struct1")
	}
	if _, ok := (myStruct2.F2[4]).(Struct2); !ok {
		t.Error("Expected F2[4] to be of type Struct2")
	}

}

//--------------------------------------------------------------------------------

func TestPointers(t *testing.T) {

	type Struct1 struct {
		Foo int
	}

	type MyStruct struct {
		F1 *Struct1
		F2 *Struct1
	}

	myStruct := MyStruct{
		F1: nil,
		F2: &Struct1{8},
	}
	buf, n, err := new(bytes.Buffer), int(0), error(nil)
	WriteBinary(myStruct, buf, &n, &err)
	if err != nil {
		t.Error("Unexpected error", err)
	}
	if hexStr := hex.EncodeToString(buf.Bytes()); hexStr !=
		"00"+"010108" {
		t.Error("Unexpected binary bytes", hexStr)
	}

	// Now, read

	myStruct2 := MyStruct{}
	ReadBinaryPtr(&myStruct2, buf, 0, &n, &err)
	if err != nil {
		t.Error("Unexpected error", err)
	}

	if myStruct2.F1 != nil {
		t.Error("Expected F1 to be nil")
	}
	if myStruct2.F2.Foo != 8 {
		t.Error("Expected F2.Foo to be 8")
	}

}

//--------------------------------------------------------------------------------

func TestUnsafe(t *testing.T) {

	type Struct1 struct {
		Foo float64
	}

	type Struct2 struct {
		Foo float64 `wire:"unsafe"`
	}

	myStruct := Struct1{5.32}
	myStruct2 := Struct2{5.32}
	buf, n, err := new(bytes.Buffer), int(0), error(nil)
	WriteBinary(myStruct, buf, &n, &err)
	if err == nil {
		t.Error("Expected error due to float without `unsafe`")
	}

	buf, n, err = new(bytes.Buffer), int(0), error(nil)
	WriteBinary(myStruct2, buf, &n, &err)
	if err != nil {
		t.Error("Unexpected error", err)
	}

	var s Struct2
	n, err = int(0), error(nil)
	ReadBinaryPtr(&s, buf, 0, &n, &err)
	if err != nil {
		t.Error("Unexpected error", err)
	}

	if s.Foo != myStruct2.Foo {
		t.Error("Expected float values to be the same. Got", s.Foo, "expected", myStruct2.Foo)
	}

}

//--------------------------------------------------------------------------------

func TestUnwrap(t *testing.T) {

	type Result interface{}
	type ConcreteResult struct{ A int }
	RegisterInterface(
		struct{ Result }{},
		ConcreteType{&ConcreteResult{}, 0x01},
	)

	type Struct1 struct {
		Result Result `json:"unwrap"`
		other  string // this should be ignored, it is unexported
		Other  string `json:"-"` // this should also be ignored
	}

	myStruct := Struct1{Result: &ConcreteResult{5}}
	buf, n, err := new(bytes.Buffer), int(0), error(nil)
	WriteJSON(myStruct, buf, &n, &err)
	if err != nil {
		t.Error("Unexpected error", err)
	}
	jsonBytes := buf.Bytes()
	if string(jsonBytes) != `[1,{"A":5}]` {
		t.Error("Unexpected jsonBytes", string(jsonBytes))
	}

	var s Struct1
	err = error(nil)
	ReadJSON(&s, jsonBytes, &err)
	if err != nil {
		t.Error("Unexpected error", err)
	}

	sConcrete, ok := s.Result.(*ConcreteResult)
	if !ok {
		t.Error("Expected struct result to be of type ConcreteResult. Got", reflect.TypeOf(s.Result))
	}

	got := sConcrete.A
	expected := myStruct.Result.(*ConcreteResult).A
	if got != expected {
		t.Error("Expected values to match. Got", got, "expected", expected)
	}

}
