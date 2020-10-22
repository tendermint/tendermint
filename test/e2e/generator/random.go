package main

import (
	"math/rand"
	"sort"
)

// combinations takes input in the form of a map of item lists, and returns a
// list of all combinations of each item for each key. E.g.:
//
// {"foo": [1, 2, 3], "bar": [4, 5, 6]}
//
// Will return the following maps:
//
// {"foo": 1, "bar": 4}
// {"foo": 1, "bar": 5}
// {"foo": 1, "bar": 6}
// {"foo": 2, "bar": 4}
// {"foo": 2, "bar": 5}
// {"foo": 2, "bar": 6}
// {"foo": 3, "bar": 4}
// {"foo": 3, "bar": 5}
// {"foo": 3, "bar": 6}
func combinations(items map[string][]interface{}) []map[string]interface{} {
	keys := []string{}
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return combiner(map[string]interface{}{}, keys, items)
}

// combiner is a utility function for combinations.
func combiner(head map[string]interface{}, pending []string, items map[string][]interface{}) []map[string]interface{} {
	if len(pending) == 0 {
		return []map[string]interface{}{head}
	}
	key, pending := pending[0], pending[1:]

	result := []map[string]interface{}{}
	for _, value := range items[key] {
		path := map[string]interface{}{}
		for k, v := range head {
			path[k] = v
		}
		path[key] = value
		result = append(result, combiner(path, pending, items)...)
	}
	return result
}

// uniformChoice chooses a single random item from the argument list, uniformly weighted.
type uniformChoice []interface{}

func (uc uniformChoice) Choose(r *rand.Rand) interface{} {
	return uc[r.Intn(len(uc))]
}

// weightedChoice chooses a single random key from a map of keys and weights.
type weightedChoice map[interface{}]uint // nolint:unused

func (wc weightedChoice) Choose(r *rand.Rand) interface{} {
	total := 0
	choices := make([]interface{}, 0, len(wc))
	for choice, weight := range wc {
		total += int(weight)
		choices = append(choices, choice)
	}

	rem := r.Intn(total)
	for _, choice := range choices {
		rem -= int(wc[choice])
		if rem <= 0 {
			return choice
		}
	}

	return nil
}

// probSetChoice picks a set of strings based on each string's probability (0-1).
type probSetChoice map[string]float64

func (pc probSetChoice) Choose(r *rand.Rand) []string {
	choices := []string{}
	for item, prob := range pc {
		if r.Float64() <= prob {
			choices = append(choices, item)
		}
	}
	return choices
}

// uniformSetChoice picks a set of strings with uniform probability, picking at least one.
type uniformSetChoice []string

func (usc uniformSetChoice) Choose(r *rand.Rand) []string {
	choices := []string{}
	indexes := r.Perm(len(usc))
	if len(indexes) > 1 {
		indexes = indexes[:1+r.Intn(len(indexes)-1)]
	}
	for _, i := range indexes {
		choices = append(choices, usc[i])
	}
	return choices
}
