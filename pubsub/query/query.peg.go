package query

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
		/* 1 condition <- <(tag ' '* ((le ' '* ((&('D' | 'd') date) | (&('T' | 't') time) | (&('0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9') number))) / (ge ' '* ((&('D' | 'd') date) | (&('T' | 't') time) | (&('0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9') number))) / ((&('=') (equal ' '* (number / time / date / value))) | (&('>') (g ' '* ((&('D' | 'd') date) | (&('T' | 't') time) | (&('0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9') number)))) | (&('<') (l ' '* ((&('D' | 'd') date) | (&('T' | 't') time) | (&('0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9') number)))) | (&('C' | 'c') (contains ' '* value)))))> */
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
								position43, tokenIndex43 := position, tokenIndex
								if !_rules[rulenumber]() {
									goto l44
								}
								goto l43
							l44:
								position, tokenIndex = position43, tokenIndex43
								if !_rules[ruletime]() {
									goto l45
								}
								goto l43
							l45:
								position, tokenIndex = position43, tokenIndex43
								if !_rules[ruledate]() {
									goto l46
								}
								goto l43
							l46:
								position, tokenIndex = position43, tokenIndex43
								if !_rules[rulevalue]() {
									goto l16
								}
							}
						l43:
							break
						case '>':
							{
								position47 := position
								if buffer[position] != rune('>') {
									goto l16
								}
								position++
								add(ruleg, position47)
							}
						l48:
							{
								position49, tokenIndex49 := position, tokenIndex
								if buffer[position] != rune(' ') {
									goto l49
								}
								position++
								goto l48
							l49:
								position, tokenIndex = position49, tokenIndex49
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
								position51 := position
								if buffer[position] != rune('<') {
									goto l16
								}
								position++
								add(rulel, position51)
							}
						l52:
							{
								position53, tokenIndex53 := position, tokenIndex
								if buffer[position] != rune(' ') {
									goto l53
								}
								position++
								goto l52
							l53:
								position, tokenIndex = position53, tokenIndex53
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
								position55 := position
								{
									position56, tokenIndex56 := position, tokenIndex
									if buffer[position] != rune('c') {
										goto l57
									}
									position++
									goto l56
								l57:
									position, tokenIndex = position56, tokenIndex56
									if buffer[position] != rune('C') {
										goto l16
									}
									position++
								}
							l56:
								{
									position58, tokenIndex58 := position, tokenIndex
									if buffer[position] != rune('o') {
										goto l59
									}
									position++
									goto l58
								l59:
									position, tokenIndex = position58, tokenIndex58
									if buffer[position] != rune('O') {
										goto l16
									}
									position++
								}
							l58:
								{
									position60, tokenIndex60 := position, tokenIndex
									if buffer[position] != rune('n') {
										goto l61
									}
									position++
									goto l60
								l61:
									position, tokenIndex = position60, tokenIndex60
									if buffer[position] != rune('N') {
										goto l16
									}
									position++
								}
							l60:
								{
									position62, tokenIndex62 := position, tokenIndex
									if buffer[position] != rune('t') {
										goto l63
									}
									position++
									goto l62
								l63:
									position, tokenIndex = position62, tokenIndex62
									if buffer[position] != rune('T') {
										goto l16
									}
									position++
								}
							l62:
								{
									position64, tokenIndex64 := position, tokenIndex
									if buffer[position] != rune('a') {
										goto l65
									}
									position++
									goto l64
								l65:
									position, tokenIndex = position64, tokenIndex64
									if buffer[position] != rune('A') {
										goto l16
									}
									position++
								}
							l64:
								{
									position66, tokenIndex66 := position, tokenIndex
									if buffer[position] != rune('i') {
										goto l67
									}
									position++
									goto l66
								l67:
									position, tokenIndex = position66, tokenIndex66
									if buffer[position] != rune('I') {
										goto l16
									}
									position++
								}
							l66:
								{
									position68, tokenIndex68 := position, tokenIndex
									if buffer[position] != rune('n') {
										goto l69
									}
									position++
									goto l68
								l69:
									position, tokenIndex = position68, tokenIndex68
									if buffer[position] != rune('N') {
										goto l16
									}
									position++
								}
							l68:
								{
									position70, tokenIndex70 := position, tokenIndex
									if buffer[position] != rune('s') {
										goto l71
									}
									position++
									goto l70
								l71:
									position, tokenIndex = position70, tokenIndex70
									if buffer[position] != rune('S') {
										goto l16
									}
									position++
								}
							l70:
								add(rulecontains, position55)
							}
						l72:
							{
								position73, tokenIndex73 := position, tokenIndex
								if buffer[position] != rune(' ') {
									goto l73
								}
								position++
								goto l72
							l73:
								position, tokenIndex = position73, tokenIndex73
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
		/* 2 tag <- <<(!((&('<') '<') | (&('>') '>') | (&('=') '=') | (&('"') '"') | (&(')') ')') | (&('(') '(') | (&('\\') '\\') | (&('\r') '\r') | (&('\n') '\n') | (&('\t') '\t') | (&(' ') ' ')) .)+>> */
		nil,
		/* 3 value <- <<(!((&('<') '<') | (&('>') '>') | (&('=') '=') | (&('"') '"') | (&(')') ')') | (&('(') '(') | (&('\\') '\\') | (&('\r') '\r') | (&('\n') '\n') | (&('\t') '\t') | (&(' ') ' ')) .)+>> */
		func() bool {
			position75, tokenIndex75 := position, tokenIndex
			{
				position76 := position
				{
					position77 := position
					{
						position80, tokenIndex80 := position, tokenIndex
						{
							switch buffer[position] {
							case '<':
								if buffer[position] != rune('<') {
									goto l80
								}
								position++
								break
							case '>':
								if buffer[position] != rune('>') {
									goto l80
								}
								position++
								break
							case '=':
								if buffer[position] != rune('=') {
									goto l80
								}
								position++
								break
							case '"':
								if buffer[position] != rune('"') {
									goto l80
								}
								position++
								break
							case ')':
								if buffer[position] != rune(')') {
									goto l80
								}
								position++
								break
							case '(':
								if buffer[position] != rune('(') {
									goto l80
								}
								position++
								break
							case '\\':
								if buffer[position] != rune('\\') {
									goto l80
								}
								position++
								break
							case '\r':
								if buffer[position] != rune('\r') {
									goto l80
								}
								position++
								break
							case '\n':
								if buffer[position] != rune('\n') {
									goto l80
								}
								position++
								break
							case '\t':
								if buffer[position] != rune('\t') {
									goto l80
								}
								position++
								break
							default:
								if buffer[position] != rune(' ') {
									goto l80
								}
								position++
								break
							}
						}

						goto l75
					l80:
						position, tokenIndex = position80, tokenIndex80
					}
					if !matchDot() {
						goto l75
					}
				l78:
					{
						position79, tokenIndex79 := position, tokenIndex
						{
							position82, tokenIndex82 := position, tokenIndex
							{
								switch buffer[position] {
								case '<':
									if buffer[position] != rune('<') {
										goto l82
									}
									position++
									break
								case '>':
									if buffer[position] != rune('>') {
										goto l82
									}
									position++
									break
								case '=':
									if buffer[position] != rune('=') {
										goto l82
									}
									position++
									break
								case '"':
									if buffer[position] != rune('"') {
										goto l82
									}
									position++
									break
								case ')':
									if buffer[position] != rune(')') {
										goto l82
									}
									position++
									break
								case '(':
									if buffer[position] != rune('(') {
										goto l82
									}
									position++
									break
								case '\\':
									if buffer[position] != rune('\\') {
										goto l82
									}
									position++
									break
								case '\r':
									if buffer[position] != rune('\r') {
										goto l82
									}
									position++
									break
								case '\n':
									if buffer[position] != rune('\n') {
										goto l82
									}
									position++
									break
								case '\t':
									if buffer[position] != rune('\t') {
										goto l82
									}
									position++
									break
								default:
									if buffer[position] != rune(' ') {
										goto l82
									}
									position++
									break
								}
							}

							goto l79
						l82:
							position, tokenIndex = position82, tokenIndex82
						}
						if !matchDot() {
							goto l79
						}
						goto l78
					l79:
						position, tokenIndex = position79, tokenIndex79
					}
					add(rulePegText, position77)
				}
				add(rulevalue, position76)
			}
			return true
		l75:
			position, tokenIndex = position75, tokenIndex75
			return false
		},
		/* 4 number <- <<('0' / ([1-9] digit* ('.' digit*)?))>> */
		func() bool {
			position84, tokenIndex84 := position, tokenIndex
			{
				position85 := position
				{
					position86 := position
					{
						position87, tokenIndex87 := position, tokenIndex
						if buffer[position] != rune('0') {
							goto l88
						}
						position++
						goto l87
					l88:
						position, tokenIndex = position87, tokenIndex87
						if c := buffer[position]; c < rune('1') || c > rune('9') {
							goto l84
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
						{
							position91, tokenIndex91 := position, tokenIndex
							if buffer[position] != rune('.') {
								goto l91
							}
							position++
						l93:
							{
								position94, tokenIndex94 := position, tokenIndex
								if !_rules[ruledigit]() {
									goto l94
								}
								goto l93
							l94:
								position, tokenIndex = position94, tokenIndex94
							}
							goto l92
						l91:
							position, tokenIndex = position91, tokenIndex91
						}
					l92:
					}
				l87:
					add(rulePegText, position86)
				}
				add(rulenumber, position85)
			}
			return true
		l84:
			position, tokenIndex = position84, tokenIndex84
			return false
		},
		/* 5 digit <- <[0-9]> */
		func() bool {
			position95, tokenIndex95 := position, tokenIndex
			{
				position96 := position
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l95
				}
				position++
				add(ruledigit, position96)
			}
			return true
		l95:
			position, tokenIndex = position95, tokenIndex95
			return false
		},
		/* 6 time <- <(('t' / 'T') ('i' / 'I') ('m' / 'M') ('e' / 'E') ' ' <(year '-' month '-' day 'T' digit digit ':' digit digit ':' digit digit ((('-' / '+') digit digit ':' digit digit) / 'Z'))>)> */
		func() bool {
			position97, tokenIndex97 := position, tokenIndex
			{
				position98 := position
				{
					position99, tokenIndex99 := position, tokenIndex
					if buffer[position] != rune('t') {
						goto l100
					}
					position++
					goto l99
				l100:
					position, tokenIndex = position99, tokenIndex99
					if buffer[position] != rune('T') {
						goto l97
					}
					position++
				}
			l99:
				{
					position101, tokenIndex101 := position, tokenIndex
					if buffer[position] != rune('i') {
						goto l102
					}
					position++
					goto l101
				l102:
					position, tokenIndex = position101, tokenIndex101
					if buffer[position] != rune('I') {
						goto l97
					}
					position++
				}
			l101:
				{
					position103, tokenIndex103 := position, tokenIndex
					if buffer[position] != rune('m') {
						goto l104
					}
					position++
					goto l103
				l104:
					position, tokenIndex = position103, tokenIndex103
					if buffer[position] != rune('M') {
						goto l97
					}
					position++
				}
			l103:
				{
					position105, tokenIndex105 := position, tokenIndex
					if buffer[position] != rune('e') {
						goto l106
					}
					position++
					goto l105
				l106:
					position, tokenIndex = position105, tokenIndex105
					if buffer[position] != rune('E') {
						goto l97
					}
					position++
				}
			l105:
				if buffer[position] != rune(' ') {
					goto l97
				}
				position++
				{
					position107 := position
					if !_rules[ruleyear]() {
						goto l97
					}
					if buffer[position] != rune('-') {
						goto l97
					}
					position++
					if !_rules[rulemonth]() {
						goto l97
					}
					if buffer[position] != rune('-') {
						goto l97
					}
					position++
					if !_rules[ruleday]() {
						goto l97
					}
					if buffer[position] != rune('T') {
						goto l97
					}
					position++
					if !_rules[ruledigit]() {
						goto l97
					}
					if !_rules[ruledigit]() {
						goto l97
					}
					if buffer[position] != rune(':') {
						goto l97
					}
					position++
					if !_rules[ruledigit]() {
						goto l97
					}
					if !_rules[ruledigit]() {
						goto l97
					}
					if buffer[position] != rune(':') {
						goto l97
					}
					position++
					if !_rules[ruledigit]() {
						goto l97
					}
					if !_rules[ruledigit]() {
						goto l97
					}
					{
						position108, tokenIndex108 := position, tokenIndex
						{
							position110, tokenIndex110 := position, tokenIndex
							if buffer[position] != rune('-') {
								goto l111
							}
							position++
							goto l110
						l111:
							position, tokenIndex = position110, tokenIndex110
							if buffer[position] != rune('+') {
								goto l109
							}
							position++
						}
					l110:
						if !_rules[ruledigit]() {
							goto l109
						}
						if !_rules[ruledigit]() {
							goto l109
						}
						if buffer[position] != rune(':') {
							goto l109
						}
						position++
						if !_rules[ruledigit]() {
							goto l109
						}
						if !_rules[ruledigit]() {
							goto l109
						}
						goto l108
					l109:
						position, tokenIndex = position108, tokenIndex108
						if buffer[position] != rune('Z') {
							goto l97
						}
						position++
					}
				l108:
					add(rulePegText, position107)
				}
				add(ruletime, position98)
			}
			return true
		l97:
			position, tokenIndex = position97, tokenIndex97
			return false
		},
		/* 7 date <- <(('d' / 'D') ('a' / 'A') ('t' / 'T') ('e' / 'E') ' ' <(year '-' month '-' day)>)> */
		func() bool {
			position112, tokenIndex112 := position, tokenIndex
			{
				position113 := position
				{
					position114, tokenIndex114 := position, tokenIndex
					if buffer[position] != rune('d') {
						goto l115
					}
					position++
					goto l114
				l115:
					position, tokenIndex = position114, tokenIndex114
					if buffer[position] != rune('D') {
						goto l112
					}
					position++
				}
			l114:
				{
					position116, tokenIndex116 := position, tokenIndex
					if buffer[position] != rune('a') {
						goto l117
					}
					position++
					goto l116
				l117:
					position, tokenIndex = position116, tokenIndex116
					if buffer[position] != rune('A') {
						goto l112
					}
					position++
				}
			l116:
				{
					position118, tokenIndex118 := position, tokenIndex
					if buffer[position] != rune('t') {
						goto l119
					}
					position++
					goto l118
				l119:
					position, tokenIndex = position118, tokenIndex118
					if buffer[position] != rune('T') {
						goto l112
					}
					position++
				}
			l118:
				{
					position120, tokenIndex120 := position, tokenIndex
					if buffer[position] != rune('e') {
						goto l121
					}
					position++
					goto l120
				l121:
					position, tokenIndex = position120, tokenIndex120
					if buffer[position] != rune('E') {
						goto l112
					}
					position++
				}
			l120:
				if buffer[position] != rune(' ') {
					goto l112
				}
				position++
				{
					position122 := position
					if !_rules[ruleyear]() {
						goto l112
					}
					if buffer[position] != rune('-') {
						goto l112
					}
					position++
					if !_rules[rulemonth]() {
						goto l112
					}
					if buffer[position] != rune('-') {
						goto l112
					}
					position++
					if !_rules[ruleday]() {
						goto l112
					}
					add(rulePegText, position122)
				}
				add(ruledate, position113)
			}
			return true
		l112:
			position, tokenIndex = position112, tokenIndex112
			return false
		},
		/* 8 year <- <(('1' / '2') digit digit digit)> */
		func() bool {
			position123, tokenIndex123 := position, tokenIndex
			{
				position124 := position
				{
					position125, tokenIndex125 := position, tokenIndex
					if buffer[position] != rune('1') {
						goto l126
					}
					position++
					goto l125
				l126:
					position, tokenIndex = position125, tokenIndex125
					if buffer[position] != rune('2') {
						goto l123
					}
					position++
				}
			l125:
				if !_rules[ruledigit]() {
					goto l123
				}
				if !_rules[ruledigit]() {
					goto l123
				}
				if !_rules[ruledigit]() {
					goto l123
				}
				add(ruleyear, position124)
			}
			return true
		l123:
			position, tokenIndex = position123, tokenIndex123
			return false
		},
		/* 9 month <- <(('0' / '1') digit)> */
		func() bool {
			position127, tokenIndex127 := position, tokenIndex
			{
				position128 := position
				{
					position129, tokenIndex129 := position, tokenIndex
					if buffer[position] != rune('0') {
						goto l130
					}
					position++
					goto l129
				l130:
					position, tokenIndex = position129, tokenIndex129
					if buffer[position] != rune('1') {
						goto l127
					}
					position++
				}
			l129:
				if !_rules[ruledigit]() {
					goto l127
				}
				add(rulemonth, position128)
			}
			return true
		l127:
			position, tokenIndex = position127, tokenIndex127
			return false
		},
		/* 10 day <- <(((&('3') '3') | (&('2') '2') | (&('1') '1') | (&('0') '0')) digit)> */
		func() bool {
			position131, tokenIndex131 := position, tokenIndex
			{
				position132 := position
				{
					switch buffer[position] {
					case '3':
						if buffer[position] != rune('3') {
							goto l131
						}
						position++
						break
					case '2':
						if buffer[position] != rune('2') {
							goto l131
						}
						position++
						break
					case '1':
						if buffer[position] != rune('1') {
							goto l131
						}
						position++
						break
					default:
						if buffer[position] != rune('0') {
							goto l131
						}
						position++
						break
					}
				}

				if !_rules[ruledigit]() {
					goto l131
				}
				add(ruleday, position132)
			}
			return true
		l131:
			position, tokenIndex = position131, tokenIndex131
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
