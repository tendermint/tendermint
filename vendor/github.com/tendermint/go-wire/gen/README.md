# Codegen for Interface Wrappers

We often want to wrap interfaces in structs to help with wire and json serialization.  However, there is a lot of boilerplate, so I made this tool.  Look at `go-crypto/pubkeyinner_holder.go` for an example of the boilerplate ...

## Setup (one-time)

Before you can run the codegen, you need to install the proper tooling on your system. Just go to the most recent `develop` of `go-wire` and type:

```bash
make tools
```

## Testing it out...

Let's see if this works now... go to `go-crypto` and checkout `data-codegen` (if it has not yet been merged into `develop`).  Now, to prepare and execute the generator, type:

```bash
make prepgen
make codegen
```

If you see no errors, it works!  You can verify by adding some random comments in `pubkeyinner_holder.go` and running `make codegen` again.  It will re-generate the code and they will disappear.

## Adding it to your repo

The best time to use it is when you create the interface... otherwise it is a bit of a dance to port code without causing duplicate names and uncompilable code.  If this is you, talk to Frey.  But best to use this when you start writing all the interfaces.

First, go to the directory that has the interface you want to wrap, and create the following file as `_gen.go`:

```Go
package main

import (
  _ "github.com/tendermint/go-wire/gen"
)
```

Now, go to the interface you want to wrap and place a comment before it.  Like:

```Go
// +gen holder:"Food,Impl[Bling,*Fuzz]"
type FooInner interface {
  Bar() int
}
```

Let's disect what this line means...

```Go
// +gen holder:"STRUCT_NAME,Impl[IMPLEMENATION_STRUCTS...],(JSON_NAMES,...)"
```

If you run it with nothing, just `// +gen holder`, it will create a surrounding struct with the name `FooInnerHolder` and register no implementations. This is rarely desired.

If you just add a name, it will apply the name to the surrounding struct.  So `// +gen holder:"Food"` leads to...

```Go
type Food Struct {
  FooInner `json:"unwrap"`
}
```

It provides all the helper methods, which is great, but you still have to register the implementations.  If you want to use codegen for this, then add a tag called `Impl[]` and include the class names inside the brackets.  Prepend `*` if the pointer receiver is what fulfills the interface, so we generate the proper bindings. This will add a `Wrap()` method to those implementations and register them with go-wire (and data) for easy serialization and deserialization.

## Running it in your repo

This will run code from `github.com/tendermin/go-wire/gen` to do the actual codegen, and knowing go, it will look in your vendor directory if you have one... so update glide and make sure you have a recent version of `go-wire:develop` in your vendor dir.

Then... just type `gen` in the same directory... suddenly a brand new file will appear called `fooinner_holder.go` or whatever the interface name was that we wrapped.

Enjoy!

One nice bonus, is if we add methods to the codegen templates, you just have to update go-wire in your vendor dir and re-run `gen` to get the new code.
