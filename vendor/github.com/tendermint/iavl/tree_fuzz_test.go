package iavl

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/tendermint/tmlibs/db"

	cmn "github.com/tendermint/tmlibs/common"
)

// This file implement fuzz testing by generating programs and then running
// them. If an error occurs, the program that had the error is printed.

// A program is a list of instructions.
type program struct {
	instructions []instruction
}

func (p *program) Execute(tree *VersionedTree) (err error) {
	var errLine int

	defer func() {
		if r := recover(); r != nil {
			var str string

			for i, instr := range p.instructions {
				prefix := "   "
				if i == errLine {
					prefix = ">> "
				}
				str += prefix + instr.String() + "\n"
			}
			err = errors.Errorf("Program panicked with: %s\n%s", r, str)
		}
	}()

	for i, instr := range p.instructions {
		errLine = i
		instr.Execute(tree)
	}
	return
}

func (p *program) addInstruction(i instruction) {
	p.instructions = append(p.instructions, i)
}

func (prog *program) size() int {
	return len(prog.instructions)
}

type instruction struct {
	op      string
	k, v    []byte
	version uint64
}

func (i instruction) Execute(tree *VersionedTree) {
	switch i.op {
	case "SET":
		tree.Set(i.k, i.v)
	case "REMOVE":
		tree.Remove(i.k)
	case "SAVE":
		tree.SaveVersion(i.version)
	case "DELETE":
		tree.DeleteVersion(i.version)
	default:
		panic("Unrecognized op: " + i.op)
	}
}

func (i instruction) String() string {
	if i.version > 0 {
		return fmt.Sprintf("%-8s %-8s %-8s %-8d", i.op, i.k, i.v, i.version)
	}
	return fmt.Sprintf("%-8s %-8s %-8s", i.op, i.k, i.v)
}

// Generate a random program of the given size.
func genRandomProgram(size int) *program {
	p := &program{}
	nextVersion := 1

	for p.size() < size {
		k, v := []byte(cmn.RandStr(1)), []byte(cmn.RandStr(1))

		switch cmn.RandInt() % 7 {
		case 0, 1, 2:
			p.addInstruction(instruction{op: "SET", k: k, v: v})
		case 3, 4:
			p.addInstruction(instruction{op: "REMOVE", k: k})
		case 5:
			p.addInstruction(instruction{op: "SAVE", version: uint64(nextVersion)})
			nextVersion++
		case 6:
			if rv := cmn.RandInt() % nextVersion; rv < nextVersion && rv > 0 {
				p.addInstruction(instruction{op: "DELETE", version: uint64(rv)})
			}
		}
	}
	return p
}

// Generate many programs and run them.
func TestVersionedTreeFuzz(t *testing.T) {
	maxIterations := testFuzzIterations
	progsPerIteration := 100000
	iterations := 0

	for size := 5; iterations < maxIterations; size++ {
		for i := 0; i < progsPerIteration/size; i++ {
			tree := NewVersionedTree(0, db.NewMemDB())
			program := genRandomProgram(size)
			err := program.Execute(tree)
			if err != nil {
				t.Fatalf("Error after %d iterations (size %d): %s\n%s", iterations, size, err.Error(), tree.String())
			}
			iterations++
		}
	}
}
