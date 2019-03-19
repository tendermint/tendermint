package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPaginationPage(t *testing.T) {

	cases := []struct {
		totalCount int
		perPage    int
		page       int
		newPage    int
	}{
		{0, 0, 1, 1},

		{0, 10, 0, 1},
		{0, 10, 1, 1},
		{0, 10, 2, 1},

		{5, 10, -1, 1},
		{5, 10, 0, 1},
		{5, 10, 1, 1},
		{5, 10, 2, 1},
		{5, 10, 2, 1},

		{5, 5, 1, 1},
		{5, 5, 2, 1},
		{5, 5, 3, 1},

		{5, 3, 2, 2},
		{5, 3, 3, 2},

		{5, 2, 2, 2},
		{5, 2, 3, 3},
		{5, 2, 4, 3},
	}

	for _, c := range cases {
		p := validatePage(c.page, c.perPage, c.totalCount)
		assert.Equal(t, c.newPage, p, fmt.Sprintf("%v", c))
	}

}

func TestPaginationPerPage(t *testing.T) {
	cases := []struct {
		totalCount int
		perPage    int
		newPerPage int
	}{
		{5, 0, defaultPerPage},
		{5, 1, 1},
		{5, 2, 2},
		{5, defaultPerPage, defaultPerPage},
		{5, maxPerPage - 1, maxPerPage - 1},
		{5, maxPerPage, maxPerPage},
		{5, maxPerPage + 1, maxPerPage},
	}

	for _, c := range cases {
		p := validatePerPage(c.perPage)
		assert.Equal(t, c.newPerPage, p, fmt.Sprintf("%v", c))
	}
}
