package query_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/libs/pubsub/query"
)

// TODO: fuzzy testing?
func TestParser(t *testing.T) {
	cases := []struct {
		query string
		valid bool
	}{
		{"tm.events.type='NewBlock'", true},
		{"tm.events.type = 'NewBlock'", true},
		{"tm.events.name = ''", true},
		{"tm.events.type='TIME'", true},
		{"tm.events.type='DATE'", true},
		{"tm.events.type='='", true},
		{"tm.events.type='TIME", false},
		{"tm.events.type=TIME'", false},
		{"tm.events.type==", false},
		{"tm.events.type=NewBlock", false},
		{">==", false},
		{"tm.events.type 'NewBlock' =", false},
		{"tm.events.type>'NewBlock'", false},
		{"", false},
		{"=", false},
		{"='NewBlock'", false},
		{"tm.events.type=", false},

		{"tm.events.typeNewBlock", false},
		{"tm.events.type'NewBlock'", false},
		{"'NewBlock'", false},
		{"NewBlock", false},
		{"", false},

		{"tm.events.type='NewBlock' AND abci.account.name='Igor'", true},
		{"tm.events.type='NewBlock' AND", false},
		{"tm.events.type='NewBlock' AN", false},
		{"tm.events.type='NewBlock' AN tm.events.type='NewBlockHeader'", false},
		{"AND tm.events.type='NewBlock' ", false},

		{"abci.account.name CONTAINS 'Igor'", true},

		{"tx.date > DATE 2013-05-03", true},
		{"tx.date < DATE 2013-05-03", true},
		{"tx.date <= DATE 2013-05-03", true},
		{"tx.date >= DATE 2013-05-03", true},
		{"tx.date >= DAT 2013-05-03", false},
		{"tx.date <= DATE2013-05-03", false},
		{"tx.date <= DATE -05-03", false},
		{"tx.date >= DATE 20130503", false},
		{"tx.date >= DATE 2013+01-03", false},
		// incorrect year, month, day
		{"tx.date >= DATE 0013-01-03", false},
		{"tx.date >= DATE 2013-31-03", false},
		{"tx.date >= DATE 2013-01-83", false},

		{"tx.date > TIME 2013-05-03T14:45:00+07:00", true},
		{"tx.date < TIME 2013-05-03T14:45:00-02:00", true},
		{"tx.date <= TIME 2013-05-03T14:45:00Z", true},
		{"tx.date >= TIME 2013-05-03T14:45:00Z", true},
		{"tx.date >= TIME2013-05-03T14:45:00Z", false},
		{"tx.date = IME 2013-05-03T14:45:00Z", false},
		{"tx.date = TIME 2013-05-:45:00Z", false},
		{"tx.date >= TIME 2013-05-03T14:45:00", false},
		{"tx.date >= TIME 0013-00-00T14:45:00Z", false},
		{"tx.date >= TIME 2013+05=03T14:45:00Z", false},

		{"account.balance=100", true},
		{"account.balance >= 200", true},
		{"account.balance >= -300", false},
		{"account.balance >>= 400", false},
		{"account.balance=33.22.1", false},

		{"hash='136E18F7E4C348B780CF873A0BF43922E5BAFA63'", true},
		{"hash=136E18F7E4C348B780CF873A0BF43922E5BAFA63", false},
	}

	for _, c := range cases {
		_, err := query.New(c.query)
		if c.valid {
			assert.NoErrorf(t, err, "Query was '%s'", c.query)
		} else {
			assert.Errorf(t, err, "Query was '%s'", c.query)
		}
	}
}
