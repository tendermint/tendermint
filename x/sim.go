package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"
)

var (
	seed     = flag.Int64("seed", 0, "Seed (if 0, use time)")
	numPubs  = flag.Int("pubs", 10, "Number of publishers")
	numSteps = flag.Int("steps", 10, "Number of simulation steps")
	pubA2D   = flag.Float64("pa2d", 0.5, "Probability of active publisher becoming dormant")
	pubD2A   = flag.Float64("pd2a", 0.1, "Probability of dormant publisher becoming active")
	pubInit  = flag.Float64("init", 0.5, "Fraction of initial publishers that are active")
	subRate  = flag.Float64("sub-rate", 1, "Subscriber processing rate")
)

type task struct {
	active     bool
	pa2d, pd2a float64
}

func newTask(pa2d, pd2a float64, init bool) *task {
	return &task{active: init, pa2d: pa2d, pd2a: pd2a}
}

func (t *task) update(roll float64) {
	t.active = (t.active && roll > t.pa2d) || (!t.active && roll <= t.pd2a)
}

func main() {
	flag.Parse()
	if *seed == 0 {
		*seed = time.Now().Unix()
	}
	log.Printf("Seed: %d", *seed)
	rng := rand.New(rand.NewSource(*seed))

	log.Printf("Initializing %d publishers", *numPubs)

	var numActive int
	pubs := make([]*task, *numPubs)
	for i := 0; i < len(pubs); i++ {
		pubs[i] = newTask(*pubA2D, *pubD2A, rng.Float64() <= *pubInit)
		if pubs[i].active {
			numActive++
		}
	}
	var sub float64

	var queueLen, numArrived, numServiced, numExcess int
	fmt.Println("STEP\tQLEN\tADDED\tREMOVED\tARRIVED\tSERVICED")
	fmt.Printf("%d\t%d\t%d\t%d\t%d\t%d\n", 0, queueLen, 0, 0, numArrived, numServiced)
	for step := 1; step <= *numSteps; step++ {
		numActive = 0
		for _, pub := range pubs {
			if pub.active {
				numActive++
			}
			pub.update(rng.Float64())
		}
		queueLen += numActive
		numArrived += numActive

		var numRemoved int
		sub += *subRate
		if sub > 1 {
			fl := math.Floor(sub)
			numRemoved = int(fl)
			if numRemoved > queueLen {
				numExcess += (numRemoved - queueLen)
				numRemoved = queueLen
			}
			sub -= fl
			queueLen -= numRemoved
		}
		numServiced += numRemoved
		fmt.Printf("%d\t%d\t%d\t%d\t%d\t%d\n",
			step, queueLen, numActive, numRemoved, numArrived, numServiced)
	}
	fmt.Printf("END\t%d\t%d\t%d\t%d\t%d\n", queueLen, 0, 0, numArrived, numServiced)
	fmt.Printf("Effective arrival rate: %f\n", float64(numArrived)/float64(*numSteps))
	fmt.Printf("Effective service rate: %f\n", float64(numServiced+numExcess)/float64(*numSteps))
}
