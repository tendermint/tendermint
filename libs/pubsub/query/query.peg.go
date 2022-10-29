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
	ruleexists
	rulele
	rulege
	rulel
	ruleg
	rulePegText

	rulePre
	ruleIn
	ruleSuf
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
	"exists",
	"le",
	"ge",
	"l",
	"g",
	"PegText",

	"Pre_",
	"_In_",
	"_Suf",
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(depth int, buffer string) {
	for node != nil {
		for c := 0; c < depth; c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[node.pegRule], strconv.Quote(string(([]rune(buffer)[node.begin:node.end]))))
		if node.up != nil {
			node.up.print(depth+1, buffer)
		}
		node = node.next
	}
}

func (node *node32) Print(buffer string) {
	node.print(0, buffer)
}

type element struct {
	node *node32
	down *element
}

/* ${@} bit structure for abstract syntax tree */
type token32 struct {
	pegRule
	begin, end, next uint32
}

func (t *token32) isZero() bool {
	return t.pegRule == ruleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token32) isParentOf(u token32) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token32) getToken32() token32 {
	return token32{pegRule: t.pegRule, begin: uint32(t.begin), end: uint32(t.end), next: uint32(t.next)}
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", rul3s[t.pegRule], t.begin, t.end, t.next)
}

type tokens32 struct {
	tree    []token32
	ordered [][]token32
}

func (t *tokens32) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) Order() [][]token32 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int32, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.pegRule == ruleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token32, len(depths)), make([]token32, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = uint32(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type state32 struct {
	token32
	depths []int32
	leaf   bool
}

func (t *tokens32) AST() *node32 {
	tokens := t.Tokens()
	stack := &element{node: &node32{token32: <-tokens}}
	for token := range tokens {
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
	return stack.node
}

func (t *tokens32) PreOrder() (<-chan state32, [][]token32) {
	s, ordered := make(chan state32, 6), t.Order()
	go func() {
		var states [8]state32
		for i := range states {
			states[i].depths = make([]int32, len(ordered))
		}
		depths, state, depth := make([]int32, len(ordered)), 0, 1
		write := func(t token32, leaf bool) {
			S := states[state]
			state, S.pegRule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.pegRule, t.begin, t.end, uint32(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token32 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token32{pegRule: ruleIn, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token32{pegRule: rulePre, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.pegRule != ruleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.pegRule != ruleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token32{pegRule: ruleSuf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens32) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", rul3s[token.pegRule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", rul3s[token.pegRule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", rul3s[token.pegRule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[token.pegRule], strconv.Quote(string(([]rune(buffer)[token.begin:token.end]))))
	}
}

func (t *tokens32) Add(rule pegRule, begin, end, depth uint32, index int) {
	t.tree[index] = token32{pegRule: rule, begin: uint32(begin), end: uint32(end), next: uint32(depth)}
}

func (t *tokens32) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.getToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens32) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].getToken32()
		}
	}
	return tokens
}

func (t *tokens32) Expand(index int) {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
}

type QueryParser struct {
	Buffer string
	buffer []rune
	rules  [21]func() bool
	Parse  func(rule ...int) error
	Reset  func()
	Pretty bool
	tokens32
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
	p.tokens32.PrintSyntaxTree(p.Buffer)
}

func (p *QueryParser) Highlighter() {
	p.PrintSyntax()
}

func (p *QueryParser) Init() {
	p.buffer = []rune(p.Buffer)
	if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != endSymbol {
		p.buffer = append(p.buffer, endSymbol)
	}

	tree := tokens32{tree: make([]token32, math.MaxInt16)}
	var max token32
	position, depth, tokenIndex, buffer, _rules := uint32(0), uint32(0), 0, p.buffer, p.rules

	p.Parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokens32 = tree
		if matches {
			p.trim(tokenIndex)
			return nil
		}
		return &parseError{p, max}
	}

	p.Reset = func() {
		position, tokenIndex, depth = 0, 0, 0
	}

	add := func(rule pegRule, begin uint32) {
		tree.Expand(tokenIndex)
		tree.Add(rule, begin, position, depth, tokenIndex)
		tokenIndex++
		if begin != position && position > max.end {
			max = token32{rule, begin, position, depth}
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
			position0, tokenIndex0, depth0 := position, tokenIndex, depth
			{
				position1 := position
				depth++
				if buffer[position] != rune('"') {
					goto l0
				}
				position++
				if !_rules[rulecondition]() {
					goto l0
				}
			l2:
				{
					position3, tokenIndex3, depth3 := position, tokenIndex, depth
					if buffer[position] != rune(' ') {
						goto l3
					}
					position++
				l4:
					{
						position5, tokenIndex5, depth5 := position, tokenIndex, depth
						if buffer[position] != rune(' ') {
							goto l5
						}
						position++
						goto l4
					l5:
						position, tokenIndex, depth = position5, tokenIndex5, depth5
					}
					{
						position6 := position
						depth++
						{
							position7, tokenIndex7, depth7 := position, tokenIndex, depth
							if buffer[position] != rune('a') {
								goto l8
							}
							position++
							goto l7
						l8:
							position, tokenIndex, depth = position7, tokenIndex7, depth7
							if buffer[position] != rune('A') {
								goto l3
							}
							position++
						}
					l7:
						{
							position9, tokenIndex9, depth9 := position, tokenIndex, depth
							if buffer[position] != rune('n') {
								goto l10
							}
							position++
							goto l9
						l10:
							position, tokenIndex, depth = position9, tokenIndex9, depth9
							if buffer[position] != rune('N') {
								goto l3
							}
							position++
						}
					l9:
						{
							position11, tokenIndex11, depth11 := position, tokenIndex, depth
							if buffer[position] != rune('d') {
								goto l12
							}
							position++
							goto l11
						l12:
							position, tokenIndex, depth = position11, tokenIndex11, depth11
							if buffer[position] != rune('D') {
								goto l3
							}
							position++
						}
					l11:
						depth--
						add(ruleand, position6)
					}
					if buffer[position] != rune(' ') {
						goto l3
					}
					position++
				l13:
					{
						position14, tokenIndex14, depth14 := position, tokenIndex, depth
						if buffer[position] != rune(' ') {
							goto l14
						}
						position++
						goto l13
					l14:
						position, tokenIndex, depth = position14, tokenIndex14, depth14
					}
					if !_rules[rulecondition]() {
						goto l3
					}
					goto l2
				l3:
					position, tokenIndex, depth = position3, tokenIndex3, depth3
				}
				if buffer[position] != rune('"') {
					goto l0
				}
				position++
				{
					position15, tokenIndex15, depth15 := position, tokenIndex, depth
					if !matchDot() {
						goto l15
					}
					goto l0
				l15:
					position, tokenIndex, depth = position15, tokenIndex15, depth15
				}
				depth--
				add(rulee, position1)
			}
			return true
		l0:
			position, tokenIndex, depth = position0, tokenIndex0, depth0
			return false
		},
		/* 1 condition <- <(tag ' '* ((le ' '* ((&('D' | 'd') date) | (&('T' | 't') time) | (&('0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9') number))) / (ge ' '* ((&('D' | 'd') date) | (&('T' | 't') time) | (&('0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9') number))) / ((&('E' | 'e') exists) | (&('=') (equal ' '* ((&('\'') value) | (&('D' | 'd') date) | (&('T' | 't') time) | (&('0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9') number)))) | (&('>') (g ' '* ((&('D' | 'd') date) | (&('T' | 't') time) | (&('0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9') number)))) | (&('<') (l ' '* ((&('D' | 'd') date) | (&('T' | 't') time) | (&('0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9') number)))) | (&('C' | 'c') (contains ' '* value)))))> */
		func() bool {
			position16, tokenIndex16, depth16 := position, tokenIndex, depth
			{
				position17 := position
				depth++
				{
					position18 := position
					depth++
					{
						position19 := position
						depth++
						{
							position22, tokenIndex22, depth22 := position, tokenIndex, depth
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
							position, tokenIndex, depth = position22, tokenIndex22, depth22
						}
						if !matchDot() {
							goto l16
						}
					l20:
						{
							position21, tokenIndex21, depth21 := position, tokenIndex, depth
							{
								position24, tokenIndex24, depth24 := position, tokenIndex, depth
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
								position, tokenIndex, depth = position24, tokenIndex24, depth24
							}
							if !matchDot() {
								goto l21
							}
							goto l20
						l21:
							position, tokenIndex, depth = position21, tokenIndex21, depth21
						}
						depth--
						add(rulePegText, position19)
					}
					depth--
					add(ruletag, position18)
				}
			l26:
				{
					position27, tokenIndex27, depth27 := position, tokenIndex, depth
					if buffer[position] != rune(' ') {
						goto l27
					}
					position++
					goto l26
				l27:
					position, tokenIndex, depth = position27, tokenIndex27, depth27
				}
				{
					position28, tokenIndex28, depth28 := position, tokenIndex, depth
					{
						position30 := position
						depth++
						if buffer[position] != rune('<') {
							goto l29
						}
						position++
						if buffer[position] != rune('=') {
							goto l29
						}
						position++
						depth--
						add(rulele, position30)
					}
				l31:
					{
						position32, tokenIndex32, depth32 := position, tokenIndex, depth
						if buffer[position] != rune(' ') {
							goto l32
						}
						position++
						goto l31
					l32:
						position, tokenIndex, depth = position32, tokenIndex32, depth32
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
					position, tokenIndex, depth = position28, tokenIndex28, depth28
					{
						position35 := position
						depth++
						if buffer[position] != rune('>') {
							goto l34
						}
						position++
						if buffer[position] != rune('=') {
							goto l34
						}
						position++
						depth--
						add(rulege, position35)
					}
				l36:
					{
						position37, tokenIndex37, depth37 := position, tokenIndex, depth
						if buffer[position] != rune(' ') {
							goto l37
						}
						position++
						goto l36
					l37:
						position, tokenIndex, depth = position37, tokenIndex37, depth37
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
					position, tokenIndex, depth = position28, tokenIndex28, depth28
					{
						switch buffer[position] {
						case 'E', 'e':
							{
								position40 := position
								depth++
								{
									position41, tokenIndex41, depth41 := position, tokenIndex, depth
									if buffer[position] != rune('e') {
										goto l42
									}
									position++
									goto l41
								l42:
									position, tokenIndex, depth = position41, tokenIndex41, depth41
									if buffer[position] != rune('E') {
										goto l16
									}
									position++
								}
							l41:
								{
									position43, tokenIndex43, depth43 := position, tokenIndex, depth
									if buffer[position] != rune('x') {
										goto l44
									}
									position++
									goto l43
								l44:
									position, tokenIndex, depth = position43, tokenIndex43, depth43
									if buffer[position] != rune('X') {
										goto l16
									}
									position++
								}
							l43:
								{
									position45, tokenIndex45, depth45 := position, tokenIndex, depth
									if buffer[position] != rune('i') {
										goto l46
									}
									position++
									goto l45
								l46:
									position, tokenIndex, depth = position45, tokenIndex45, depth45
									if buffer[position] != rune('I') {
										goto l16
									}
									position++
								}
							l45:
								{
									position47, tokenIndex47, depth47 := position, tokenIndex, depth
									if buffer[position] != rune('s') {
										goto l48
									}
									position++
									goto l47
								l48:
									position, tokenIndex, depth = position47, tokenIndex47, depth47
									if buffer[position] != rune('S') {
										goto l16
									}
									position++
								}
							l47:
								{
									position49, tokenIndex49, depth49 := position, tokenIndex, depth
									if buffer[position] != rune('t') {
										goto l50
									}
									position++
									goto l49
								l50:
									position, tokenIndex, depth = position49, tokenIndex49, depth49
									if buffer[position] != rune('T') {
										goto l16
									}
									position++
								}
							l49:
								{
									position51, tokenIndex51, depth51 := position, tokenIndex, depth
									if buffer[position] != rune('s') {
										goto l52
									}
									position++
									goto l51
								l52:
									position, tokenIndex, depth = position51, tokenIndex51, depth51
									if buffer[position] != rune('S') {
										goto l16
									}
									position++
								}
							l51:
								depth--
								add(ruleexists, position40)
							}
							break
						case '=':
							{
								position53 := position
								depth++
								if buffer[position] != rune('=') {
									goto l16
								}
								position++
								depth--
								add(ruleequal, position53)
							}
						l54:
							{
								position55, tokenIndex55, depth55 := position, tokenIndex, depth
								if buffer[position] != rune(' ') {
									goto l55
								}
								position++
								goto l54
							l55:
								position, tokenIndex, depth = position55, tokenIndex55, depth55
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
								position57 := position
								depth++
								if buffer[position] != rune('>') {
									goto l16
								}
								position++
								depth--
								add(ruleg, position57)
							}
						l58:
							{
								position59, tokenIndex59, depth59 := position, tokenIndex, depth
								if buffer[position] != rune(' ') {
									goto l59
								}
								position++
								goto l58
							l59:
								position, tokenIndex, depth = position59, tokenIndex59, depth59
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
								position61 := position
								depth++
								if buffer[position] != rune('<') {
									goto l16
								}
								position++
								depth--
								add(rulel, position61)
							}
						l62:
							{
								position63, tokenIndex63, depth63 := position, tokenIndex, depth
								if buffer[position] != rune(' ') {
									goto l63
								}
								position++
								goto l62
							l63:
								position, tokenIndex, depth = position63, tokenIndex63, depth63
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
								position65 := position
								depth++
								{
									position66, tokenIndex66, depth66 := position, tokenIndex, depth
									if buffer[position] != rune('c') {
										goto l67
									}
									position++
									goto l66
								l67:
									position, tokenIndex, depth = position66, tokenIndex66, depth66
									if buffer[position] != rune('C') {
										goto l16
									}
									position++
								}
							l66:
								{
									position68, tokenIndex68, depth68 := position, tokenIndex, depth
									if buffer[position] != rune('o') {
										goto l69
									}
									position++
									goto l68
								l69:
									position, tokenIndex, depth = position68, tokenIndex68, depth68
									if buffer[position] != rune('O') {
										goto l16
									}
									position++
								}
							l68:
								{
									position70, tokenIndex70, depth70 := position, tokenIndex, depth
									if buffer[position] != rune('n') {
										goto l71
									}
									position++
									goto l70
								l71:
									position, tokenIndex, depth = position70, tokenIndex70, depth70
									if buffer[position] != rune('N') {
										goto l16
									}
									position++
								}
							l70:
								{
									position72, tokenIndex72, depth72 := position, tokenIndex, depth
									if buffer[position] != rune('t') {
										goto l73
									}
									position++
									goto l72
								l73:
									position, tokenIndex, depth = position72, tokenIndex72, depth72
									if buffer[position] != rune('T') {
										goto l16
									}
									position++
								}
							l72:
								{
									position74, tokenIndex74, depth74 := position, tokenIndex, depth
									if buffer[position] != rune('a') {
										goto l75
									}
									position++
									goto l74
								l75:
									position, tokenIndex, depth = position74, tokenIndex74, depth74
									if buffer[position] != rune('A') {
										goto l16
									}
									position++
								}
							l74:
								{
									position76, tokenIndex76, depth76 := position, tokenIndex, depth
									if buffer[position] != rune('i') {
										goto l77
									}
									position++
									goto l76
								l77:
									position, tokenIndex, depth = position76, tokenIndex76, depth76
									if buffer[position] != rune('I') {
										goto l16
									}
									position++
								}
							l76:
								{
									position78, tokenIndex78, depth78 := position, tokenIndex, depth
									if buffer[position] != rune('n') {
										goto l79
									}
									position++
									goto l78
								l79:
									position, tokenIndex, depth = position78, tokenIndex78, depth78
									if buffer[position] != rune('N') {
										goto l16
									}
									position++
								}
							l78:
								{
									position80, tokenIndex80, depth80 := position, tokenIndex, depth
									if buffer[position] != rune('s') {
										goto l81
									}
									position++
									goto l80
								l81:
									position, tokenIndex, depth = position80, tokenIndex80, depth80
									if buffer[position] != rune('S') {
										goto l16
									}
									position++
								}
							l80:
								depth--
								add(rulecontains, position65)
							}
						l82:
							{
								position83, tokenIndex83, depth83 := position, tokenIndex, depth
								if buffer[position] != rune(' ') {
									goto l83
								}
								position++
								goto l82
							l83:
								position, tokenIndex, depth = position83, tokenIndex83, depth83
							}
							if !_rules[rulevalue]() {
								goto l16
							}
							break
						}
					}

				}
			l28:
				depth--
				add(rulecondition, position17)
			}
			return true
		l16:
			position, tokenIndex, depth = position16, tokenIndex16, depth16
			return false
		},
		/* 2 tag <- <<(!((&('<') '<') | (&('>') '>') | (&('=') '=') | (&('\'') '\'') | (&('"') '"') | (&(')') ')') | (&('(') '(') | (&('\\') '\\') | (&('\r') '\r') | (&('\n') '\n') | (&('\t') '\t') | (&(' ') ' ')) .)+>> */
		nil,
		/* 3 value <- <<('\'' (!('"' / '\'') .)* '\'')>> */
		func() bool {
			position85, tokenIndex85, depth85 := position, tokenIndex, depth
			{
				position86 := position
				depth++
				{
					position87 := position
					depth++
					if buffer[position] != rune('\'') {
						goto l85
					}
					position++
				l88:
					{
						position89, tokenIndex89, depth89 := position, tokenIndex, depth
						{
							position90, tokenIndex90, depth90 := position, tokenIndex, depth
							{
								position91, tokenIndex91, depth91 := position, tokenIndex, depth
								if buffer[position] != rune('"') {
									goto l92
								}
								position++
								goto l91
							l92:
								position, tokenIndex, depth = position91, tokenIndex91, depth91
								if buffer[position] != rune('\'') {
									goto l90
								}
								position++
							}
						l91:
							goto l89
						l90:
							position, tokenIndex, depth = position90, tokenIndex90, depth90
						}
						if !matchDot() {
							goto l89
						}
						goto l88
					l89:
						position, tokenIndex, depth = position89, tokenIndex89, depth89
					}
					if buffer[position] != rune('\'') {
						goto l85
					}
					position++
					depth--
					add(rulePegText, position87)
				}
				depth--
				add(rulevalue, position86)
			}
			return true
		l85:
			position, tokenIndex, depth = position85, tokenIndex85, depth85
			return false
		},
		/* 4 number <- <<('0' / ([1-9] digit* ('.' digit*)?))>> */
		func() bool {
			position93, tokenIndex93, depth93 := position, tokenIndex, depth
			{
				position94 := position
				depth++
				{
					position95 := position
					depth++
					{
						position96, tokenIndex96, depth96 := position, tokenIndex, depth
						if buffer[position] != rune('0') {
							goto l97
						}
						position++
						goto l96
					l97:
						position, tokenIndex, depth = position96, tokenIndex96, depth96
						if c := buffer[position]; c < rune('1') || c > rune('9') {
							goto l93
						}
						position++
					l98:
						{
							position99, tokenIndex99, depth99 := position, tokenIndex, depth
							if !_rules[ruledigit]() {
								goto l99
							}
							goto l98
						l99:
							position, tokenIndex, depth = position99, tokenIndex99, depth99
						}
						{
							position100, tokenIndex100, depth100 := position, tokenIndex, depth
							if buffer[position] != rune('.') {
								goto l100
							}
							position++
						l102:
							{
								position103, tokenIndex103, depth103 := position, tokenIndex, depth
								if !_rules[ruledigit]() {
									goto l103
								}
								goto l102
							l103:
								position, tokenIndex, depth = position103, tokenIndex103, depth103
							}
							goto l101
						l100:
							position, tokenIndex, depth = position100, tokenIndex100, depth100
						}
					l101:
					}
				l96:
					depth--
					add(rulePegText, position95)
				}
				depth--
				add(rulenumber, position94)
			}
			return true
		l93:
			position, tokenIndex, depth = position93, tokenIndex93, depth93
			return false
		},
		/* 5 digit <- <[0-9]> */
		func() bool {
			position104, tokenIndex104, depth104 := position, tokenIndex, depth
			{
				position105 := position
				depth++
				if c := buffer[position]; c < rune('0') || c > rune('9') {
					goto l104
				}
				position++
				depth--
				add(ruledigit, position105)
			}
			return true
		l104:
			position, tokenIndex, depth = position104, tokenIndex104, depth104
			return false
		},
		/* 6 time <- <(('t' / 'T') ('i' / 'I') ('m' / 'M') ('e' / 'E') ' ' <(year '-' month '-' day 'T' digit digit ':' digit digit ':' digit digit ((('-' / '+') digit digit ':' digit digit) / 'Z'))>)> */
		func() bool {
			position106, tokenIndex106, depth106 := position, tokenIndex, depth
			{
				position107 := position
				depth++
				{
					position108, tokenIndex108, depth108 := position, tokenIndex, depth
					if buffer[position] != rune('t') {
						goto l109
					}
					position++
					goto l108
				l109:
					position, tokenIndex, depth = position108, tokenIndex108, depth108
					if buffer[position] != rune('T') {
						goto l106
					}
					position++
				}
			l108:
				{
					position110, tokenIndex110, depth110 := position, tokenIndex, depth
					if buffer[position] != rune('i') {
						goto l111
					}
					position++
					goto l110
				l111:
					position, tokenIndex, depth = position110, tokenIndex110, depth110
					if buffer[position] != rune('I') {
						goto l106
					}
					position++
				}
			l110:
				{
					position112, tokenIndex112, depth112 := position, tokenIndex, depth
					if buffer[position] != rune('m') {
						goto l113
					}
					position++
					goto l112
				l113:
					position, tokenIndex, depth = position112, tokenIndex112, depth112
					if buffer[position] != rune('M') {
						goto l106
					}
					position++
				}
			l112:
				{
					position114, tokenIndex114, depth114 := position, tokenIndex, depth
					if buffer[position] != rune('e') {
						goto l115
					}
					position++
					goto l114
				l115:
					position, tokenIndex, depth = position114, tokenIndex114, depth114
					if buffer[position] != rune('E') {
						goto l106
					}
					position++
				}
			l114:
				if buffer[position] != rune(' ') {
					goto l106
				}
				position++
				{
					position116 := position
					depth++
					if !_rules[ruleyear]() {
						goto l106
					}
					if buffer[position] != rune('-') {
						goto l106
					}
					position++
					if !_rules[rulemonth]() {
						goto l106
					}
					if buffer[position] != rune('-') {
						goto l106
					}
					position++
					if !_rules[ruleday]() {
						goto l106
					}
					if buffer[position] != rune('T') {
						goto l106
					}
					position++
					if !_rules[ruledigit]() {
						goto l106
					}
					if !_rules[ruledigit]() {
						goto l106
					}
					if buffer[position] != rune(':') {
						goto l106
					}
					position++
					if !_rules[ruledigit]() {
						goto l106
					}
					if !_rules[ruledigit]() {
						goto l106
					}
					if buffer[position] != rune(':') {
						goto l106
					}
					position++
					if !_rules[ruledigit]() {
						goto l106
					}
					if !_rules[ruledigit]() {
						goto l106
					}
					{
						position117, tokenIndex117, depth117 := position, tokenIndex, depth
						{
							position119, tokenIndex119, depth119 := position, tokenIndex, depth
							if buffer[position] != rune('-') {
								goto l120
							}
							position++
							goto l119
						l120:
							position, tokenIndex, depth = position119, tokenIndex119, depth119
							if buffer[position] != rune('+') {
								goto l118
							}
							position++
						}
					l119:
						if !_rules[ruledigit]() {
							goto l118
						}
						if !_rules[ruledigit]() {
							goto l118
						}
						if buffer[position] != rune(':') {
							goto l118
						}
						position++
						if !_rules[ruledigit]() {
							goto l118
						}
						if !_rules[ruledigit]() {
							goto l118
						}
						goto l117
					l118:
						position, tokenIndex, depth = position117, tokenIndex117, depth117
						if buffer[position] != rune('Z') {
							goto l106
						}
						position++
					}
				l117:
					depth--
					add(rulePegText, position116)
				}
				depth--
				add(ruletime, position107)
			}
			return true
		l106:
			position, tokenIndex, depth = position106, tokenIndex106, depth106
			return false
		},
		/* 7 date <- <(('d' / 'D') ('a' / 'A') ('t' / 'T') ('e' / 'E') ' ' <(year '-' month '-' day)>)> */
		func() bool {
			position121, tokenIndex121, depth121 := position, tokenIndex, depth
			{
				position122 := position
				depth++
				{
					position123, tokenIndex123, depth123 := position, tokenIndex, depth
					if buffer[position] != rune('d') {
						goto l124
					}
					position++
					goto l123
				l124:
					position, tokenIndex, depth = position123, tokenIndex123, depth123
					if buffer[position] != rune('D') {
						goto l121
					}
					position++
				}
			l123:
				{
					position125, tokenIndex125, depth125 := position, tokenIndex, depth
					if buffer[position] != rune('a') {
						goto l126
					}
					position++
					goto l125
				l126:
					position, tokenIndex, depth = position125, tokenIndex125, depth125
					if buffer[position] != rune('A') {
						goto l121
					}
					position++
				}
			l125:
				{
					position127, tokenIndex127, depth127 := position, tokenIndex, depth
					if buffer[position] != rune('t') {
						goto l128
					}
					position++
					goto l127
				l128:
					position, tokenIndex, depth = position127, tokenIndex127, depth127
					if buffer[position] != rune('T') {
						goto l121
					}
					position++
				}
			l127:
				{
					position129, tokenIndex129, depth129 := position, tokenIndex, depth
					if buffer[position] != rune('e') {
						goto l130
					}
					position++
					goto l129
				l130:
					position, tokenIndex, depth = position129, tokenIndex129, depth129
					if buffer[position] != rune('E') {
						goto l121
					}
					position++
				}
			l129:
				if buffer[position] != rune(' ') {
					goto l121
				}
				position++
				{
					position131 := position
					depth++
					if !_rules[ruleyear]() {
						goto l121
					}
					if buffer[position] != rune('-') {
						goto l121
					}
					position++
					if !_rules[rulemonth]() {
						goto l121
					}
					if buffer[position] != rune('-') {
						goto l121
					}
					position++
					if !_rules[ruleday]() {
						goto l121
					}
					depth--
					add(rulePegText, position131)
				}
				depth--
				add(ruledate, position122)
			}
			return true
		l121:
			position, tokenIndex, depth = position121, tokenIndex121, depth121
			return false
		},
		/* 8 year <- <(('1' / '2') digit digit digit)> */
		func() bool {
			position132, tokenIndex132, depth132 := position, tokenIndex, depth
			{
				position133 := position
				depth++
				{
					position134, tokenIndex134, depth134 := position, tokenIndex, depth
					if buffer[position] != rune('1') {
						goto l135
					}
					position++
					goto l134
				l135:
					position, tokenIndex, depth = position134, tokenIndex134, depth134
					if buffer[position] != rune('2') {
						goto l132
					}
					position++
				}
			l134:
				if !_rules[ruledigit]() {
					goto l132
				}
				if !_rules[ruledigit]() {
					goto l132
				}
				if !_rules[ruledigit]() {
					goto l132
				}
				depth--
				add(ruleyear, position133)
			}
			return true
		l132:
			position, tokenIndex, depth = position132, tokenIndex132, depth132
			return false
		},
		/* 9 month <- <(('0' / '1') digit)> */
		func() bool {
			position136, tokenIndex136, depth136 := position, tokenIndex, depth
			{
				position137 := position
				depth++
				{
					position138, tokenIndex138, depth138 := position, tokenIndex, depth
					if buffer[position] != rune('0') {
						goto l139
					}
					position++
					goto l138
				l139:
					position, tokenIndex, depth = position138, tokenIndex138, depth138
					if buffer[position] != rune('1') {
						goto l136
					}
					position++
				}
			l138:
				if !_rules[ruledigit]() {
					goto l136
				}
				depth--
				add(rulemonth, position137)
			}
			return true
		l136:
			position, tokenIndex, depth = position136, tokenIndex136, depth136
			return false
		},
		/* 10 day <- <(((&('3') '3') | (&('2') '2') | (&('1') '1') | (&('0') '0')) digit)> */
		func() bool {
			position140, tokenIndex140, depth140 := position, tokenIndex, depth
			{
				position141 := position
				depth++
				{
					switch buffer[position] {
					case '3':
						if buffer[position] != rune('3') {
							goto l140
						}
						position++
						break
					case '2':
						if buffer[position] != rune('2') {
							goto l140
						}
						position++
						break
					case '1':
						if buffer[position] != rune('1') {
							goto l140
						}
						position++
						break
					default:
						if buffer[position] != rune('0') {
							goto l140
						}
						position++
						break
					}
				}

				if !_rules[ruledigit]() {
					goto l140
				}
				depth--
				add(ruleday, position141)
			}
			return true
		l140:
			position, tokenIndex, depth = position140, tokenIndex140, depth140
			return false
		},
		/* 11 and <- <(('a' / 'A') ('n' / 'N') ('d' / 'D'))> */
		nil,
		/* 12 equal <- <'='> */
		nil,
		/* 13 contains <- <(('c' / 'C') ('o' / 'O') ('n' / 'N') ('t' / 'T') ('a' / 'A') ('i' / 'I') ('n' / 'N') ('s' / 'S'))> */
		nil,
		/* 14 exists <- <(('e' / 'E') ('x' / 'X') ('i' / 'I') ('s' / 'S') ('t' / 'T') ('s' / 'S'))> */
		nil,
		/* 15 le <- <('<' '=')> */
		nil,
		/* 16 ge <- <('>' '=')> */
		nil,
		/* 17 l <- <'<'> */
		nil,
		/* 18 g <- <'>'> */
		nil,
		nil,
	}
	p.rules = _rules
}
