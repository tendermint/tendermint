Wire Protocol
=============

The `Tendermint wire protocol <https://github.com/tendermint/go-wire>`__
encodes data in `c-style binary <#binary>`__ and `JSON <#json>`__ form.

Supported types
---------------

-  Primitive types
-  ``uint8`` (aka ``byte``), ``uint16``, ``uint32``, ``uint64``
-  ``int8``, ``int16``, ``int32``, ``int64``
-  ``uint``, ``int``: variable length (un)signed integers
-  ``string``, ``[]byte``
-  ``time``
-  Derived types
-  structs
-  var-length arrays of a particular type
-  fixed-length arrays of a particular type
-  interfaces: registered union types preceded by a ``type byte``
-  pointers

Binary
------

**Fixed-length primitive types** are encoded with 1,2,3, or 4 big-endian
bytes. - ``uint8`` (aka ``byte``), ``uint16``, ``uint32``, ``uint64``:
takes 1,2,3, and 4 bytes respectively - ``int8``, ``int16``, ``int32``,
``int64``: takes 1,2,3, and 4 bytes respectively - ``time``: ``int64``
representation of nanoseconds since epoch

**Variable-length integers** are encoded with a single leading byte
representing the length of the following big-endian bytes. For signed
negative integers, the most significant bit of the leading byte is a 1.

-  ``uint``: 1-byte length prefixed variable-size (0 ~ 255 bytes)
   unsigned integers
-  ``int``: 1-byte length prefixed variable-size (0 ~ 127 bytes) signed
   integers

NOTE: While the number 0 (zero) is encoded with a single byte ``x00``,
the number 1 (one) takes two bytes to represent: ``x0101``. This isn't
the most efficient representation, but the rules are easier to remember.

+---------------+----------------+----------------+
| number        | binary         | binary ``int`` |
|               | ``uint``       |                |
+===============+================+================+
| 0             | ``x00``        | ``x00``        |
+---------------+----------------+----------------+
| 1             | ``x0101``      | ``x0101``      |
+---------------+----------------+----------------+
| 2             | ``x0102``      | ``x0102``      |
+---------------+----------------+----------------+
| 256           | ``x020100``    | ``x020100``    |
+---------------+----------------+----------------+
| 2^(127\ *8)-1 | ``x800100...`` | overflow       |
| \|            |                |                |
| ``x7FFFFF...` |                |                |
| `             |                |                |
| \|            |                |                |
| ``x7FFFFF...` |                |                |
| `             |                |                |
| \| \|         |                |                |
| 2^(127*\ 8)   |                |                |
+---------------+----------------+----------------+
| 2^(255\*8)-1  |
| \|            |
| ``xFFFFFF...` |
| `             |
| \| overflow   |
| \| \| -1 \|   |
| n/a \|        |
| ``x8101`` \|  |
| \| -2 \| n/a  |
| \| ``x8102``  |
| \| \| -256 \| |
| n/a \|        |
| ``x820100``   |
| \|            |
+---------------+----------------+----------------+

**Structures** are encoded by encoding the field values in order of
declaration.

.. code:: go

    type Foo struct {
        MyString string
        MyUint32 uint32
    }
    var foo = Foo{"626172", math.MaxUint32}

    /* The binary representation of foo:
     0103626172FFFFFFFF
     0103:               `int` encoded length of string, here 3
         626172:         3 bytes of string "bar"
               FFFFFFFF: 4 bytes of uint32 MaxUint32
    */

**Variable-length arrays** are encoded with a leading ``int`` denoting
the length of the array followed by the binary representation of the
items. **Fixed-length arrays** are similar but aren't preceded by the
leading ``int``.

.. code:: go

    foos := []Foo{foo, foo}

    /* The binary representation of foos:
     01020103626172FFFFFFFF0103626172FFFFFFFF
     0102:                                     `int` encoded length of array, here 2
         0103626172FFFFFFFF:                   the first `foo`
                           0103626172FFFFFFFF: the second `foo`
    */

    foos := [2]Foo{foo, foo} // fixed-length array

    /* The binary representation of foos:
     0103626172FFFFFFFF0103626172FFFFFFFF
     0103626172FFFFFFFF:                   the first `foo`
                       0103626172FFFFFFFF: the second `foo`
    */

**Interfaces** can represent one of any number of concrete types. The
concrete types of an interface must first be declared with their
corresponding ``type byte``. An interface is then encoded with the
leading ``type byte``, then the binary encoding of the underlying
concrete type.

NOTE: The byte ``x00`` is reserved for the ``nil`` interface value and
``nil`` pointer values.

.. code:: go

    type Animal interface{}
    type Dog uint32
    type Cat string

    RegisterInterface(
        struct{ Animal }{},          // Convenience for referencing the 'Animal' interface
        ConcreteType{Dog(0),  0x01}, // Register the byte 0x01 to denote a Dog
        ConcreteType{Cat(""), 0x02}, // Register the byte 0x02 to denote a Cat
    )

    var animal Animal = Dog(02)

    /* The binary representation of animal:
     010102
     01:     the type byte for a `Dog`
       0102: the bytes of Dog(02)
    */

**Pointers** are encoded with a single leading byte ``x00`` for ``nil``
pointers, otherwise encoded with a leading byte ``x01`` followed by the
binary encoding of the value pointed to.

NOTE: It's easy to convert pointer types into interface types, since the
``type byte`` ``x00`` is always ``nil``.

JSON
----

The JSON codec is compatible with the ```binary`` <#binary>`__ codec,
and is fairly intuitive if you're already familiar with golang's JSON
encoding. Some quirks are noted below:

-  variable-length and fixed-length bytes are encoded as uppercase
   hexadecimal strings
-  interface values are encoded as an array of two items:
   ``[type_byte, concrete_value]``
-  times are encoded as rfc2822 strings
