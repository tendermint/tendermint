// nolint
package query

//go:generate peg -inline -switch query.peg

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

const endSymbol rune = 1114112

/* The rule types inferred from the grammar are below. */
type pegRule uint8

const (
	ruleUnknown pegRule = iota
	rulee
	rulecondition
	ruletag
	rulevalue
	rulenumber
	ruledigit
	ruletime
	ruledate
	ruleyear
	rulemonth
	ruleday
	ruleand
	ruleequal
	rulecontains
	rulele
	rulege
	rulel
	ruleg
	rulePegText
)

var rul3s = [...]string{
	"Unknown",
	"e",
	"condition",
	"tag",
	"value",
	"number",
	"digit",
	"time",
	"date",
	"year",
	"month",
	"day",
	"and",
	"equal",
	"contains",
	"le",
	"ge",
	"l",
	"g",
	"PegText",
}

type token32 struct {
	pegRule
	begin, end uint32
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v", rul3s[t.pegRule], t.begin, t.end)
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(pretty bool, buffer string) {
	var print func(node *node32, depth int)
	print = func(node *node32, depth int) {
		for node != nil {
			for c := 0; c < depth; c++ {
				fmt.Printf(" ")
			}
			rule := rul3s[node.pegRule]
			quote := strconv.Quote(string(([]rune(buffer)[node.begin:node.end])))
			if !pretty {
				fmt.Printf("%v %v\n", rule, quote)
			} else {
				fmt.Printf("\x1B[34m%v\x1B[m %v\n", rule, quote)
			}
			if node.up != nil {
				print(node.up, depth+1)
			}
			node = node.next
		}
	}
	print(node, 0)
}

func (node *node32) Print(buffer string) {
	node.print(false, buffer)
}

func (node *node32) PrettyPrint(buffer string) {
	node.print(true, buffer)
}

type tokens32 struct {
	tree []token32
}

func (t *tokens32) Trim(length uint32) {
	t.tree = t.tree[:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) AST() *node32 {
	type element struct {
		node *node32
		down *element
	}
	tokens := t.Tokens()
	var stack *element
	for _, token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	if stack != nil {
		return stack.node
	}
	return nil
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	t.AST().Print(buffer)
}

func (t *tokens32) PrettyPrintSyntaxTree(buffer string) {
	t.AST().PrettyPrint(buffer)
}

func (t *tokens32) Add(rule pegRule, begin, end, index uint32) {
	if tree := t.tree; int(index) >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	t.tree[index] = token32{
		pegRule: rule,
		begin:   begin,
		end:     end,
	}
}

func (t *tokens32) Tokens() []token32 {
	return t.tree
}

type QueryParser struct {
	Buffer string
	buffer []rune
	rules  [20]func() bool
	parse  func(rule ...int) error
	reset  func()
	Pretty bool
	tokens32
}

func (p *QueryParser) Parse(rule ...int) error {
	return p.parse(rule...)
}

func (p *QueryParser) Reset() {
	p.reset()
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer []rune, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range buffer {
		if c == '\n' {
			line, symbol = line+1, 0
		} else {
			symbol++
		}
		if i == positions[j] {
			translations[positions[j]] = textPosition{line, symbol}
			for j++; j < length; j++ {
				if i != positions[j] {
					continue search
				}
			}
			break search
		}
	}

	return translations
}

type parseError struct {
	p   *QueryParser
	max token32
}

func (e *parseError) Error() string {
	tokens, error := []token32{e.max}, "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.buffer, positions)
	format := "parse error near %v (line %v symbol %v - line %v symbol %v):\n%v\n"
	if e.p.Pretty {
		format = "parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n"
	}
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf(format,
			rul3s[token.pegRule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			strconv.Quote(string(e.p.buffer[begin:end])))
	}

	return error
}

func (p *QueryParser) PrintSyntaxTree() {
	if p.Pretty {
		p.tokens32.PrettyPrintSyntaxTree(p.Buffer)
	} else {
		p.tokens32.PrintSyntaxTree(p.Buffer)
	}
}

func (p *QueryParser) Init() {
	var (
		max                  token32
		position, tokenIndex uint32
		buffer               []rune
	)
	p.reset = func() {
		max = token32{}
		position, tokenIndex = 0, 0

		p.buffer = []rune(p.Buffer)
		if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != endSymbol {
			p.buffer = append(p.buffer, endSymbol)
		}
		buffer = p.buffer
	}
	p.reset()

	_rules := p.rules
	tree := tokens32{tree: make([]token32, math.MaxInt16)}
	p.parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokens32 = tree
		if matches {
			p.Trim(tokenIndex)
			return nil
		}
		return &parseError{p, max}
	}

	add := func(rule pegRule, begin uint32) {
		tree.Add(rule, begin, position, tokenIndex)
		tokenIndex++
		if begin != position && position > max.end {
			max = token32{rule, begin, position}
		}
	}

	matchDot := func() bool {
		if buffer[position] != endSymbol {
			position++
			return true
		}
		return false
	}

	/*matchChar := func(c byte) bool {
		if buffer[position] == c {
			position++
			return true
		}
		return false
	}*/

	/*matchRange := func(lower byte, upper byte) bool {
		if c := buffer[position]; c >= lower && c <= upper {
			position++
			return true
		}
		return false
	}*/

	_rules = [...]func() bool{
		nil,
		/* 0 e <- <('"' condition (' '+ and ' '+ condition)* '"' !.)> */
		func() bool {
			position0, tokenIndex0 := position, tokenIndex
			{
				position1 := position
				if buffer[position] != rune('"') {
					goto l0
				}
				position++
				if !_rules[rulecondition]() {
					goto l0
				}
			l2:
				{
					position3, tokenIndex3 := position, tokenIndex
					if buffer[position] != rune(' ') {
						goto l3
					}
					position++
				l4:
					{
						position5, tokenIndex5 := position, tokenIndex
						if buffer[position] != rune(' ') {
							goto l5
						}
						position++
						goto l4
					l5:
						position, tokenIndex = position5, tokenIndex5
					}
					{
						position6 := position
						{
							position7, tokenIndex7 := position, tokenIndex
							if buffer[position] != rune('a') {
								goto l8
							}
							position++
							goto l7
						l8:
							position, tokenIndex = position7, tokenIndex7
							if buffer[position] != rune('A') {
								goto l3
							}
							position++
						}
					l7:
						{
							position9, tokenIndex9 := position, tokenIndex
							if buffer[position] != rune('n') {
								goto l10
							}
							position++
							goto l9
						l10:
							position, tokenIndex = position9, tokenIndex9
							if buffer[position] != rune('N') {
								goto l3
							}
							position++
						}
					l9:
						{
							position11, tokenIndex11 := position, tokenIndex
							if buffer[position] != rune('d') {
								goto l12
							}
							position++
							goto l11
						l12:
							position, tokenIndex = position11, tokenIndex11
							if buffer[position] != rune('D') {
								goto l3
							}
							position++
						}
					l11:
						add(ruleand, position6)
					}
					if buffer[position] != rune(' ') {
						goto l3
					}
					position++
				l13:
					{
						position14, tokenIndex14 := position, tokenIndex
						if buffer[position] != rune(' ') {
							goto l14
						}
						position++
						goto l13
					l14:
						position, tokenIndex = position14, tokenIndex14
					}
					if !_rules[rulecondition]() {
						goto l3
					}
					goto l2
				l3:
					position, tokenIndex = position3, tokenIndex3
				}
				if buffer[position] != rune('"') {
					goto l0
				}
				position++
				{
					position15, tokenIndex15 := position, tokenIndex
					if !matchDot() {
						goto l15
					}
					goto l0
				l15:
					position, tokenIndex = position15, tokenIndex15
				}
				add(rulee, position1)
			}
			return true
		l0:
			position, tokenIndex = position0, tokenIndex0
			return false
		},
		/* 1 condition <- <(tag ' '* ((le ' '* ((&('D' | 'd') date) | (&('T' | 't') time) | (&('0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9') number))) / (ge ' '* ((&('D' | 'd') date) | (&('T' | 't') time) | (&('0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9') number))) / ((&('=') (equal ' '* ((&('\'') value) | (&('D' | 'd') date) | (&('T' | 't') time) | (&('0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9') number)))) | (&('>') (g ' '* ((&('D' | 'd') date) | (&('T' | 't') time) | (&('0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9') number)))) | (&('<') (l ' '* ((&('D' | 'd') date) | (&('T' | 't') time) | (&('0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9') number)))) | (&('C' | 'c') (contains ' '* value)))))> */
		func() bool {
			position16, tokenIndex16 := position, tokenIndex
			{
				position17 := position
				{
					position18 := position
					{
						position19 := position
						{
							position22, tokenIndex22 := position, tokenIndex
							{
								switch buffer[position] {
								case '<':
									if buffer[position] != rune('<') {
										goto l22
									}
									position++
									break
								case '>':
									if buffer[position] != rune('>') {
										goto l22
									}
									position++
									break
								case '=':
									if buffer[position] != rune('=') {
										goto l22
									}
									position++
									break
								case '\'':
									if buffer[position] != rune('\'') {
										goto l22
									}
									position++
									break
								case '"':
									if buffer[position] != rune('"') {
										goto l22
									}
									position++
									break
								case ')':
									if buffer[position] != rune(')') {
										goto l22
									}
									position++
									break
								case '(':
									if buffer[position] != rune('(') {
										goto l22
									}
									position++
									break
								case '\\':
									if buffer[position] != rune('\\') {
										goto l22
									}
									position++
									break
								case '\r':
									if buffer[position] != rune('\r') {
										goto l22
									}
									position++
									break
								case '\n':
									if buffer[position] != rune('\n') {
										goto l22
									}
									position++
									break
								case '\t':
									if buffer[position] != rune('\t') {
										goto l22
									}
									position++
									break
								default:
									if buffer[position] != rune(' ') {
										goto l22
									}
									position++
									break
								}
							}

							goto l16
						l22:
							position, tokenIndex = position22, tokenIndex22
						}
						if !matchDot() {
							goto l16
						}
					l20:
						{
							position21, tokenIndex21 := position, tokenIndex
							{
								position24, tokenIndex24 := position, tokenIndex
								{
									switch buffer[position] {
									case '<':
										if buffer[position] != rune('<') {
											goto l24
										}
										position++
										break
									case '>':
										if buffer[position] != rune('>') {
											goto l24
										}
										position++
										break
									case '=':
										if buffer[position] != rune('=') {
											goto l24
										}
										position++
										break
									case '\'':
										if buffer[position] != rune('\'') {
											goto l24
										}
										position++
										break
									case '"':
										if buffer[position] != rune('"') {
											goto l24
										}
										position++
										break
									case ')':
										if buffer[position] != rune(')') {
											goto l24
										}
										position++
										break
									case '(':
										if buffer[position] != rune('(') {
											goto l24
										}
										position++
										break
									case '\\':
										if buffer[position] != rune('\\') {
											goto l24
										}
										position++
										break
									case '\r':
										if buffer[position] != rune('\r') {
											goto l24
										}
										position++
										break
									case '\n':
										if buffer[position] != rune('\n') {
											goto l24
										}
										position++
										break
									case '\t':
										if buffer[position] != rune('\t') {
											goto l24
										}
										position++
										break
									default:
										if buffer[position] != rune(' ') {
											goto l24
										}
										position++
										break
									}
								}

								goto l21
							l24:
								position, tokenIndex = position24, tokenIndex24
							}
							if !matchDot() {
								goto l21
							}
							goto l20
						l21:
							position, tokenIndex = position21, tokenIndex21
						}
						add(rulePegText, position19)
					}
					add(ruletag, position18)
				}
			l26:
				{
					position27, tokenIndex27 := position, tokenIndex
					if buffer[position] != rune(' ') {
						goto l27
					}
					position++
					goto l26
				l27:
					position, tokenIndex = position27, tokenIndex27
				}
				{
					position28, tokenIndex28 := position, tokenIndex
					{
						position30 := position
						if buffer[position] != rune('<') {
							goto l29
						}
						position++
						if buffer[position] != rune('=') {
							goto l29
						}
						position++
						add(rulele, position30)
					}
				l31:
					{
						position32, tokenIndex32 := position, tokenIndex
						if buffer[position] != rune(' ') {
							goto l32
						}
						position++
						goto l31
					l32:
						position, tokenIndex = position32, tokenIndex32
					}
					{
						switch buffer[position] {
						case 'D', 'd':
							if !_rules[ruledate]() {
								goto l29
							}
							break
						case 'T', 't':
							if !_rules[ruletime]() {
								goto l29
							}
							break
						default:
							if !_rules[rulenumber]() {
								goto l29
							}
							break
						}
					}

					goto l28
				l29:
					position, tokenIndex = position28, tokenIndex28
					{
						position35 := position
						if buffer[position] != rune('>') {
							goto l34
						}
						position++
						if buffer[position] != rune('=') {
							goto l34
						}
						position++
						add(rulege, position35)
					}
				l36:
					{
						position37, tokenIndex37 := position, tokenIndex
						if buffer[position] != rune(' ') {
							goto l37
						}
						position++
						goto l36
					l37:
						position, tokenIndex = position37, tokenIndex37
					}
					{
						switch buffer[position] {
						case 'D', 'd':
							if !_rules[ruledate]() {
								goto l34
							}
							break
						case 'T', 't':
							if !_rules[ruletime]() {
								goto l34
							}
							break
						default:
							if !_rules[rulenumber]() {
								goto l34
							}
							break
						}
					}

					goto l28
				l34:
					position, tokenIndex = position28, tokenIndex28
					{
						switch buffer[position] {
						case '=':
							{
								position40 := position
								if buffer[position] != rune('=') {
									goto l16
								}
								position++
								add(ruleequal, position40)
							}
						l41:
							{
								position42, tokenIndex42 := position, tokenIndex
								if buffer[position] != rune(' ') {
									goto l42
								}
								position++
								goto l41
							l42:
								position, tokenIndex = position42, tokenIndex42
							}
							{
								switch buffer[position] {
								case '\'':
									if !_rules[rulevalue]() {
										goto l16
									}
									break
								case 'D', 'd':
									if !_rules[ruledate]() {
										goto l16
									}
									break
								case 'T', 't':
									if !_rules[ruletime]() {
										goto l16
									}
									break
								default:
									if !_rules[rulenumber]() {
										goto l16
									}
									break
								}
							}

							break
						case '>':
							{
								position44 := position
								if buffer[position] != rune('>') {
									goto l16
								}
								position++
								add(ruleg, position44)
							}
						l45:
							{
								position46, tokenIndex46 := position, tokenIndex
								if buffer[position] != rune(' ') {
									goto l46
								}
								position++
								goto l45
							l46:
								position, tokenIndex = position46, tokenIndex46
							}
							{
								switch buffer[position] {
								case 'D', 'd':
									if !_rules[ruledate]() {
										goto l16
									}
									break
								case 'T', 't':
									if !_rules[ruletime]() {
										goto l16
									}
									break
								default:
									if !_rules[rulenumber]() {
										goto l16
									}
									break
								}
							}

							break
						case '<':
							{
								position48 := position
								if buffer[position] != rune('<') {
									goto l16
								}
								position++
								add(rulel, position48)
							}
						l49:
							{
								position50, tokenIndex50 := position, tokenIndex
								if buffer[position] != rune(' ') {
									goto l50
								}
								position++
								goto l49
							l50:
								position, tokenIndex = position50, tokenIndex50
							}
							{
								switch buffer[position] {
								case 'D', 'd':
									if !_rules[ruledate]() {
										goto l16
									}
									break
								case 'T', 't':
									if !_rules[ruletime]() {
										goto l16
									}
									break
								default:
									if !_rules[rulenumber]() {
										goto l16
									}
									break
								}
							}

							break
						default:
							{
								position52 := position
								{
									position53, tokenIndex53 := position, tokenIndex
									if buffer[position] != rune('c') {
										goto l54
									}
									position++
									goto l53
								l54:
									position, tokenIndex = position53, tokenIndex53
									if buffer[position] != rune('C') {
										goto l16
									}
									position++
								}
							l53:
								{
									position55, tokenIndex55 := position, tokenIndex
									if buffer[position] != rune('o') {
										goto l56
									}
									position++
									goto l55
								l56:
									position, tokenIndex = position55, tokenIndex55
									if buffer[position] != rune('O') {
										goto l16
									}
									position++
								}
							l55:
								{
									position57, tokenIndex57 := position, tokenIndex
									if buffer[position] != rune('n') {
										goto l58
									}
									position++
									goto l57
								l58:
									position, tokenIndex = position57, tokenIndex57
									if buffer[position] != rune('N') {
										goto l16
									}
									position++
								}
							l57:
								{
									position59, tokenIndex59 := position, tokenIndex
									if buffer[position] != rune('t') {
										goto l60
									}
									position++
									goto l59
								l60:
									position, tokenIndex = position59, tokenIndex59
									if buffer[position] != rune('T') {
										goto l16
									}
									position++
								}
							l59:
								{
									position61, tokenIndex61 := position, tokenIndex
									if buffer[position] != rune('a') {
										goto l62
									}
									position++
									goto l61
								l62:
									position, tokenIndex = position61, tokenIndex61
									if buffer[position] != rune('A') {
										goto l16
									}
									position++
								}
							l61:
								{
									position63, tokenIndex63 := position, tokenIndex
									if buffer[position] != rune('i') {
										goto l64
									}
									position++
									goto l63
								l64:
									position, tokenIndex = position63, tokenIndex63
									if buffer[position] != rune('I') {
										goto l16
									}
									position++
								}
							l63:
								{
									position65, tokenIndex65 := position, tokenIndex
									if buffer[position] != rune('n') {
										goto l66
									}
									position++
									goto l65
								l66:
									position, tokenIndex = position65, tokenIndex65
									if buffer[position] != rune('N') {
										goto l16
									}
									position++
								}
							l65:
								{
									position67, tokenIndex67 := position, tokenIndex
									if buffer[position] != rune('s') {
										goto l68
									}
									position++
									goto l67
								l68:
									position, tokenIndex = position67, tokenIndex67
									if buffer[position] != rune('S') {
										goto l16
									}
									position++
								}
							l67:
								add(rulecontains, position52)
							}
						l69:
							{
								position70, tokenIndex70 := position, tokenIndex
								if buffer[position] != rune(' ') {
									goto l70
								}
								position++
								goto l69
							l70:
								position, tokenIndex = position70, tokenIndex70
							}
							if !_rules[rulevalue]() {
								goto l16
							}
							break
						}
					}

				}
			l28:
				add(rulecondition, position17)
			}
			return true
		l16:
			position, tokenIndex = position16, tokenIndex16
			return false
		},
		/* 2 tag <- <<(!((&('<') '<') | (&('>') '>') | (&('=') '=') | (&('\'') '\'') | (&('"') '"') | (&(')') ')') | (&('(') '(') | (&('\\') '\\') | (&('\r') '\r') | (&('\n') '\n') | (&('\t') '\t') | (&(' ') ' ')) .)+>> */
		nil,
		/* 3 value <- <<('\'' (!('"' / '\'') .)* '\'')>> */
		func() bool {
			position72, tokenIndex72 := position, tokenIndex
			{
				position73 := position
				{
					position74 := position
					if buffer[position] != rune('\'') {
						goto l72
					}
					position++
				l75:
					{
						position76, tokenIndex76 := position, tokenIndex
						{
							position77, tokenIndex77 := position, tokenIndex
							{
								position78, tokenIndex78 := position, tokenIndex
								if buffer[position] != rune('"') {
									goto l79
								}
								position++
								goto l78
							l79:
								position, tokenIndex = position78, tokenIndex78
								if buffer[position] != rune('\'') {
									goto l77
								}
								position++
							}
						l78:
							goto l76
						l77:
							position, tokenIndex = position77, tokenIndex77
						}
						if !matchDot() {
							goto l76
						}
						goto l75
					l76:
						position, tokenIndex = position76, tokenIndex76
					}
					if buffer[position] != rune('\'') {
						goto l72
					}
					position++
					add(rulePegText, position74)
				}
				add(rulevalue, position73)
			}
			return true
		l72:
			position, tokenIndex = position72, tokenIndex72
			return false
		},
		/* 4 number <- <<('0' / ([1-9] digit* ('.' digit*)?))>> */
		func() bool {
			position80, tokenIndex80 := position, tokenIndex
			{
				position81 := position
				{
					position82 := position
					{
						position83, tokenIndex83 := position, tokenIndex
						if buffer[position] != rune('0') {
							goto l84
						}
						position++
						goto l83
					l84:
						position, tokenIndex = position83, tokenIndex83
						if c := buffer[position]; c < rune('1') || c > rune('9') {
							goto l80
						}
						position++
					l85:
						{
							position86, tokenIndex86 := position, tokenIndex
							if !_rules[ruledigit]() {
								goto l86
							}
							goto l85
						l86:
							position, tokenIndex = position86, tokenIndex86
						}
						{
							position87, tokenIndex87 := position, tokenIndex
							if buffer[position] != rune('.') {
								goto l87
							}
							position++
						l89:
							{
								position90, tokenIndex90 := position, tokenIndex
								if !_rules[ruledigit]() {
									goto l90
								}
								goto l89
							l90:
								position, tokenIndex = position90, tokenIndex90
							}
							goto l88
						l87:
							position, tokenIndex = position87, tokenIndex87
						}
					l88:
					}
				l83:
					add(rulePegText, position82)
				}
				add(rulenumber, position81)
			}
			return true
		l80:
			position, tokenIndex = position80, tokenIndex80
			return false
		},
		/* 5 digit <- <[0-9]> */
		func() bool {
			position91, tokenIndex91 := position, tokenIndex
			{
				position92 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l91
				}
				position++
				add(ruledigit, position92)
			}
			return true
		l91:
			position, tokenIndex = position91, tokenIndex91
			return false
		},
		/* 6 time <- <(('t' / 'T') ('i' / 'I') ('m' / 'M') ('e' / 'E') ' ' <(year '-' month '-' day 'T' digit digit ':' digit digit ':' digit digit ((('-' / '+') digit digit ':' digit digit) / 'Z'))>)> */
		func() bool {
			position93, tokenIndex93 := position, tokenIndex
			{
				position94 := position
				{
					position95, tokenIndex95 := position, tokenIndex
					if buffer[position] != rune('t') {
						goto l96
					}
					position++
					goto l95
				l96:
					position, tokenIndex = position95, tokenIndex95
					if buffer[position] != rune('T') {
						goto l93
					}
					position++
				}
			l95:
				{
					position97, tokenIndex97 := position, tokenIndex
					if buffer[position] != rune('i') {
						goto l98
					}
					position++
					goto l97
				l98:
					position, tokenIndex = position97, tokenIndex97
					if buffer[position] != rune('I') {
						goto l93
					}
					position++
				}
			l97:
				{
					position99, tokenIndex99 := position, tokenIndex
					if buffer[position] != rune('m') {
						goto l100
					}
					position++
					goto l99
				l100:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('M') {
						goto l93
					}
					position++
				}
			l99:
				{
					position101, tokenIndex101 := position, tokenIndex
					if buffer[position] != rune('e') {
						goto l102
					}
					position++
					goto l101
				l102:
					position, tokenIndex = position101, tokenIndex101
					if buffer[position] != rune('E') {
						goto l93
					}
					position++
				}
			l101:
				if buffer[position] != rune(' ') {
					goto l93
				}
				position++
				{
					position103 := position
					if !_rules[ruleyear]() {
						goto l93
					}
					if buffer[position] != rune('-') {
						goto l93
					}
					position++
					if !_rules[rulemonth]() {
						goto l93
					}
					if buffer[position] != rune('-') {
						goto l93
					}
					position++
					if !_rules[ruleday]() {
						goto l93
					}
					if buffer[position] != rune('T') {
						goto l93
					}
					position++
					if !_rules[ruledigit]() {
						goto l93
					}
					if !_rules[ruledigit]() {
						goto l93
					}
					if buffer[position] != rune(':') {
						goto l93
					}
					position++
					if !_rules[ruledigit]() {
						goto l93
					}
					if !_rules[ruledigit]() {
						goto l93
					}
					if buffer[position] != rune(':') {
						goto l93
					}
					position++
					if !_rules[ruledigit]() {
						goto l93
					}
					if !_rules[ruledigit]() {
						goto l93
					}
					{
						position104, tokenIndex104 := position, tokenIndex
						{
							position106, tokenIndex106 := position, tokenIndex
							if buffer[position] != rune('-') {
								goto l107
							}
							position++
							goto l106
						l107:
							position, tokenIndex = position106, tokenIndex106
							if buffer[position] != rune('+') {
								goto l105
							}
							position++
						}
					l106:
						if !_rules[ruledigit]() {
							goto l105
						}
						if !_rules[ruledigit]() {
							goto l105
						}
						if buffer[position] != rune(':') {
							goto l105
						}
						position++
						if !_rules[ruledigit]() {
							goto l105
						}
						if !_rules[ruledigit]() {
							goto l105
						}
						goto l104
					l105:
						position, tokenIndex = position104, tokenIndex104
						if buffer[position] != rune('Z') {
							goto l93
						}
						position++
					}
				l104:
					add(rulePegText, position103)
				}
				add(ruletime, position94)
			}
			return true
		l93:
			position, tokenIndex = position93, tokenIndex93
			return false
		},
		/* 7 date <- <(('d' / 'D') ('a' / 'A') ('t' / 'T') ('e' / 'E') ' ' <(year '-' month '-' day)>)> */
		func() bool {
			position108, tokenIndex108 := position, tokenIndex
			{
				position109 := position
				{
					position110, tokenIndex110 := position, tokenIndex
					if buffer[position] != rune('d') {
						goto l111
					}
					position++
					goto l110
				l111:
					position, tokenIndex = position110, tokenIndex110
					if buffer[position] != rune('D') {
						goto l108
					}
					position++
				}
			l110:
				{
					position112, tokenIndex112 := position, tokenIndex
					if buffer[position] != rune('a') {
						goto l113
					}
					position++
					goto l112
				l113:
					position, tokenIndex = position112, tokenIndex112
					if buffer[position] != rune('A') {
						goto l108
					}
					position++
				}
			l112:
				{
					position114, tokenIndex114 := position, tokenIndex
					if buffer[position] != rune('t') {
						goto l115
					}
					position++
					goto l114
				l115:
					position, tokenIndex = position114, tokenIndex114
					if buffer[position] != rune('T') {
						goto l108
					}
					position++
				}
			l114:
				{
					position116, tokenIndex116 := position, tokenIndex
					if buffer[position] != rune('e') {
						goto l117
					}
					position++
					goto l116
				l117:
					position, tokenIndex = position116, tokenIndex116
					if buffer[position] != rune('E') {
						goto l108
					}
					position++
				}
			l116:
				if buffer[position] != rune(' ') {
					goto l108
				}
				position++
				{
					position118 := position
					if !_rules[ruleyear]() {
						goto l108
					}
					if buffer[position] != rune('-') {
						goto l108
					}
					position++
					if !_rules[rulemonth]() {
						goto l108
					}
					if buffer[position] != rune('-') {
						goto l108
					}
					position++
					if !_rules[ruleday]() {
						goto l108
					}
					add(rulePegText, position118)
				}
				add(ruledate, position109)
			}
			return true
		l108:
			position, tokenIndex = position108, tokenIndex108
			return false
		},
		/* 8 year <- <(('1' / '2') digit digit digit)> */
		func() bool {
			position119, tokenIndex119 := position, tokenIndex
			{
				position120 := position
				{
					position121, tokenIndex121 := position, tokenIndex
					if buffer[position] != rune('1') {
						goto l122
					}
					position++
					goto l121
				l122:
					position, tokenIndex = position121, tokenIndex121
					if buffer[position] != rune('2') {
						goto l119
					}
					position++
				}
			l121:
				if !_rules[ruledigit]() {
					goto l119
				}
				if !_rules[ruledigit]() {
					goto l119
				}
				if !_rules[ruledigit]() {
					goto l119
				}
				add(ruleyear, position120)
			}
			return true
		l119:
			position, tokenIndex = position119, tokenIndex119
			return false
		},
		/* 9 month <- <(('0' / '1') digit)> */
		func() bool {
			position123, tokenIndex123 := position, tokenIndex
			{
				position124 := position
				{
					position125, tokenIndex125 := position, tokenIndex
					if buffer[position] != rune('0') {
						goto l126
					}
					position++
					goto l125
				l126:
					position, tokenIndex = position125, tokenIndex125
					if buffer[position] != rune('1') {
						goto l123
					}
					position++
				}
			l125:
				if !_rules[ruledigit]() {
					goto l123
				}
				add(rulemonth, position124)
			}
			return true
		l123:
			position, tokenIndex = position123, tokenIndex123
			return false
		},
		/* 10 day <- <(((&('3') '3') | (&('2') '2') | (&('1') '1') | (&('0') '0')) digit)> */
		func() bool {
			position127, tokenIndex127 := position, tokenIndex
			{
				position128 := position
				{
					switch buffer[position] {
					case '3':
						if buffer[position] != rune('3') {
							goto l127
						}
						position++
						break
					case '2':
						if buffer[position] != rune('2') {
							goto l127
						}
						position++
						break
					case '1':
						if buffer[position] != rune('1') {
							goto l127
						}
						position++
						break
					default:
						if buffer[position] != rune('0') {
							goto l127
						}
						position++
						break
					}
				}

				if !_rules[ruledigit]() {
					goto l127
				}
				add(ruleday, position128)
			}
			return true
		l127:
			position, tokenIndex = position127, tokenIndex127
			return false
		},
		/* 11 and <- <(('a' / 'A') ('n' / 'N') ('d' / 'D'))> */
		nil,
		/* 12 equal <- <'='> */
		nil,
		/* 13 contains <- <(('c' / 'C') ('o' / 'O') ('n' / 'N') ('t' / 'T') ('a' / 'A') ('i' / 'I') ('n' / 'N') ('s' / 'S'))> */
		nil,
		/* 14 le <- <('<' '=')> */
		nil,
		/* 15 ge <- <('>' '=')> */
		nil,
		/* 16 l <- <'<'> */
		nil,
		/* 17 g <- <'>'> */
		nil,
		nil,
	}
	p.rules = _rules
}
