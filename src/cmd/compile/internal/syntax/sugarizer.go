package syntax

import (
	"fmt"
)

var _visited = map[uintptr]bool{}

type rules struct {
	collector, lhs, def, index, selector, slice, typename bool
}

// a sugarizer walks a complete parse tree and applies syntactic sugar.
// it does not type check, but it keeps track of what identifiers are
// in scope
type sugarizer struct {
	errh func(error)

	rules         rules
	seenCollector bool

	scope *scope

	targets []*Name

	labelCt int

	deep int
}

func (s *sugarizer) run(errh func(error), file *File) (first error) {
	*s = sugarizer{
		errh: func(e error) {
			if first == nil {
				first = e
			}
			if errh != nil {
				errh(e)
			} else {
				panic(e)
			}
		},
	}

	s.file(file)

	return first
}

func (s *sugarizer) currentCollectLabel() *Name {
	return &Name{Value: fmt.Sprintf("___collect__end_%d__", s.labelCt)}
}

func (s *sugarizer) pushTarget(t *Name) { s.targets = append(s.targets, t) }

func (s *sugarizer) getTarget() *Name {
	// must always succeed
	return s.targets[len(s.targets)-1]
}

func (s *sugarizer) popTarget() {
	tCount := len(s.targets)
	if tCount > 0 {
		s.targets = s.targets[:tCount-1]
		return
	}
	panic("unexpected call to popTarget")
}

func (s *sugarizer) openScope() {
	if s.scope == nil {
		s.scope = &scope{names: map[string]*Name{}}
	} else {
		s.scope = &scope{names: map[string]*Name{}, parent: s.scope}
	}
}

func (s *sugarizer) closeScope() {
	if debug && s.scope == nil {
		panic("unexpected call to closeScope")
	}

	s.scope = s.scope.parent
}

// func (s *sugarizer) pushRulesCopy()             { s.pushRules(s.rules()) }
// func (s *sugarizer) pushRulesWith(add rules)    { s.pushRules(s.rules() | add) }
// func (s *sugarizer) pushRulesWithout(rem rules) { s.pushRules(s.rules() & ^rem) }
// func (s *sugarizer) pushRules(r rules)          { s.ruleState = append(s.ruleState, r) }
// func (s *sugarizer) popRules() {
// 	ruleCount := len(s.ruleState)
// 	if ruleCount > 0 {
// 		s.ruleState = s.ruleState[:ruleCount-1]
// 	}
// }

// func (s *sugarizer) rules() rules {
// 	ruleCount := len(s.ruleState)
// 	if ruleCount > 0 {
// 		return s.ruleState[ruleCount-1]
// 	}
// 	return rDefault
// }

func (s *sugarizer) error(msg string, pos Pos) {
	err := Error{Pos: pos, Msg: msg}
	if s.errh != nil {
		s.errh(err)
	} else {
		panic(err)
	}
}

func (s *sugarizer) file(f *File) {
	if s.rules != *new(rules) {
		panic("unclosed rule state")
	}

	for _, decl := range f.DeclList {
		switch real := decl.(type) {
		case *ImportDecl:
			// nothing to do
		case *TypeDecl:
			// nothing to do
		case *ConstDecl:
			// nothing to do
		case *VarDecl:
			// nothing to do
		case *FuncDecl:
			s.funcDecl(real)
		default:
			panic("unhandled decl")
		}
	}
}

func (s *sugarizer) funcDecl(f *FuncDecl) {
	if f.Body == nil {
		// ignore function preamble
		return
	}
	s.openScope()
	if f.Recv != nil && f.Recv.Name != nil && f.Recv.Name.Value != "_" {
		s.scope.set(f.Recv.Name.Value, f.Recv.Name)
	}
	f.Type = s.exprAsType(f.Type).(*FuncType)
	s.funcBody(f.Body, f.Type)
	s.closeScope()
}

func (s *sugarizer) funcBody(b *BlockStmt, t *FuncType) {
	oldDef := s.rules.def
	s.rules.def = false
	for _, parm := range t.ParamList {
		if parm.Name != nil && parm.Name.Value != "_" {
			s.scope.set(parm.Name.Value, parm.Name)
		}
	}

	for _, res := range t.ResultList {
		if res.Name != nil && res.Name.Value != "_" {
			if s.scope.get(res.Name.Value) == nil {
				s.scope.set(res.Name.Value, res.Name)
			}
		}
	}

	b.List = s.stmtList(b.List)
	s.rules.def = oldDef
}

func (s *sugarizer) stmtList(list []Stmt) []Stmt {
	result := make([]Stmt, 0, len(list)+10)

	for _, stmt := range list {
		replace, add := s.stmt(stmt)
		result = append(result, replace)
		if len(add) > 0 {
			result = append(result, add...)
		}
	}

	return result
}

//go:noinline
func (s *sugarizer) stmt(stmtArg Stmt) (replace Stmt, add []Stmt) {
	s.deep++
	if stmtArg != nil && s.deep > 20 {
		fmt.Printf("|rdepth=%d| [%T]@%s\n\n", s.deep, stmtArg, stmtArg.Pos())
		if s.deep > 50 {
			panic("out of control recursion")
		}
	}
	switch real := stmtArg.(type) {
	case nil: // nothing to do
	case *EmptyStmt: // nothing to do
	case *LabeledStmt:
		s.scope.set(real.Label.Value, real.Label)
		real.Stmt, _ = s.stmt(real.Stmt)

	case *BlockStmt:
		real.List = s.stmtList(real.List)

	case *ExprStmt:
		real.X = s.expr(real.X)

	case *SendStmt:
		real.Chan = s.expr(real.Chan)
		real.Value = s.expr(real.Value)

	case *DeclStmt:
		for _, d := range real.DeclList {
			switch reald := d.(type) {
			case *ConstDecl:
				for _, n := range reald.NameList {
					if n.Value == "_!" {
						s.error("cannot declare _!", n.Pos())
					}
				}

				reald.Values = s.expr(reald.Values)

			case *VarDecl:
				for _, n := range reald.NameList {
					if n.Value == "_!" {
						s.error("cannot declare _!", n.Pos())
					} else {
						s.scope.set(n.Value, n)
					}
				}

				reald.Values = s.expr(reald.Values)

			case *TypeDecl:
				if reald.Name.Value == "_!" {
					s.error("cannot declare _!", reald.Name.Pos())
				}
			}
		}

	case *AssignStmt:
		oldDef := s.rules.def
		if real.Op == Def {
			s.rules.def = true
		}

		real.Lhs = s.checkLHS(real.Lhs)
		if s.seenCollector {
			s.seenCollector = false
			add = []Stmt{
				&IfStmt{
					stmt: stmt{node: node{pos: real.Pos()}},
					Cond: &Operation{Op: Eql, X: s.getTarget(), Y: &Name{Value: "nil"}},
					Then: &BlockStmt{
						List: []Stmt{
							&BranchStmt{
								Tok:   _Goto,
								Label: s.currentCollectLabel(),
							},
						},
					},
				},
			}
		}

		real.Rhs = s.checkRHS(real.Rhs)

		s.rules.def = oldDef

	case *BranchStmt:
		real.Target, _ = s.stmt(real.Target)

	case *CallStmt:
		real.Call = s.expr(real.Call).(*CallExpr)

	case *ReturnStmt:
		real.Results = s.expr(real.Results)

	case *IfStmt:
		s.openScope()
		real.Cond = s.expr(real.Cond)
		if real.Init != nil {
			simple, _ := s.stmt(real.Init)
			real.Init = simple.(SimpleStmt)
		}

		s.openScope()
		blockStmt, _ := s.stmt(real.Then)
		real.Then = blockStmt.(*BlockStmt)
		s.closeScope()

		if real.Else != nil {
			s.openScope()
			real.Else, _ = s.stmt(real.Else)
			s.closeScope()
		}
		s.closeScope()

	case *ForStmt:
		s.openScope()

		if real.Cond != nil {
			real.Cond = s.expr(real.Cond)
		}

		if real.Init != nil {
			simple, _ := s.stmt(real.Init)
			real.Init = simple.(SimpleStmt)
		}

		if real.Post != nil {
			simple, _ := s.stmt(real.Post)
			real.Post = simple.(SimpleStmt)
		}

		blockStmt, _ := s.stmt(real.Body)
		real.Body = blockStmt.(*BlockStmt)
		s.closeScope()

	case *SwitchStmt:
		s.openScope()
		if real.Init != nil {
			simple, _ := s.stmt(real.Init)
			real.Init = simple.(SimpleStmt)
		}

		real.Tag = s.expr(real.Tag)

		for _, cc := range real.Body {
			cc.Cases = s.expr(cc.Cases)
			s.openScope()
			cc.Body = s.stmtList(cc.Body)
			s.closeScope()
		}
		s.closeScope()

	case *SelectStmt:
		for _, cc := range real.Body {
			s.openScope()
			if cc.Comm != nil {
				simple, _ := s.stmt(cc.Comm)
				cc.Comm = simple.(SimpleStmt)
			}

			cc.Body = s.stmtList(cc.Body)
			s.closeScope()
		}

	case *CollectStmt:
		s.labelCt++

		if real.Target.Value == "_!" {
			s.error("illegal use of \"_!\" as collect target", real.Target.Pos())
		}

		if !s.scope.available(real.Target.Value) {
			s.error(
				fmt.Sprintf("undeclared identifier %q", real.Target.Value),
				real.Target.Pos(),
			)
		}

		result := &BlockStmt{
			Rbrace: real.Body.Rbrace,
			stmt:   stmt{node{pos: real.Pos()}},
		}

		s.pushTarget(real.Target)
		oldCol := s.rules.collector
		s.rules.collector = true
		s.openScope()
		result.List = s.stmtList(real.Body.List)
		replace = result
		add = []Stmt{
			&LabeledStmt{
				Label: s.currentCollectLabel(),
				Stmt:  &EmptyStmt{},
			},
		}
		s.closeScope()
		s.rules.collector = oldCol
		s.popTarget()

	case *RangeClause:

	default:
		panic("unhandled stmt")
	}

	s.deep--

	if replace == nil {
		return stmtArg, add
	}

	return replace, add
}

func (s *sugarizer) checkLHS(lhs Expr) Expr {
	oldLhs := s.rules.lhs
	s.rules.lhs = true
	lhs = s.expr(lhs)
	s.rules.lhs = oldLhs
	return lhs
}

func (s *sugarizer) checkRHS(rhs Expr) Expr { return s.expr(rhs) }

func (s *sugarizer) exprAsType(e Expr) Expr {
	old := s.rules.typename
	s.rules.typename = true
	got := s.expr(e)
	s.rules.typename = old
	return got
}

func (s *sugarizer) exprAsValue(e Expr) Expr {
	old := s.rules.typename
	s.rules.typename = false
	got := s.expr(e)
	s.rules.typename = old
	return got
}

//go:noinline
func (s *sugarizer) expr(e Expr) Expr {
	switch real := e.(type) {
	case nil:
	case *Name:
		if real == nil {
			// do nothing
		} else if s.rules.typename {
			if real.Value == "_!" {
				s.error("_! used as type", real.Pos())
			}
		} else if s.rules.lhs && !(s.rules.index || s.rules.selector || s.rules.slice) {
			if real.Value == "_!" {
				// if we have `_!`, that means we're in a collect block
				// the parser rejects any program where this isn't true
				// (for now),  but to be safe we keep the check
				if s.rules.collector {
					s.error("cannot use _! outside of a collect block", real.Pos())
				}

				if s.rules.def {
					s.error("cannot declare _!", real.Pos())
				}

				if s.seenCollector {
					s.error("multiple _! on left side of assignment", real.Pos())
				}

				// signal that we've found one
				s.seenCollector = true
				s.deep--
				return s.getTarget()
			}
		} else {
			if real.Value == "_!" {
				if s.rules.collector {
					s.error("cannot use _! outside of collect block", real.Pos())
				}

				s.error("_! used as value", real.Pos())
			}
		}

	case *BasicLit:
	case *CompositeLit:
		real.Type = s.exprAsType(real.Type)
		for i, elt := range real.ElemList {
			real.ElemList[i] = s.expr(elt)
		}

	case *KeyValueExpr:
		real.Key = s.expr(real.Key)
		real.Value = s.expr(real.Value)

	case *ParenExpr:
		real.X = s.expr(real.X)

	case *FuncLit:
		real.Type = s.exprAsType(real.Type).(*FuncType)
		s.funcBody(real.Body, real.Type)

	case *SelectorExpr:
		oldSel := s.rules.selector
		s.rules.selector = true
		real.Sel = s.expr(real.Sel).(*Name)
		real.X = s.expr(real.X)
		s.rules.selector = oldSel

	case *IndexExpr:
		oldIdx := s.rules.index
		s.rules.index = true
		real.X = s.exprAsValue(real.X)
		real.Index = s.exprAsValue(real.Index)
		s.rules.index = oldIdx

	case *SliceExpr:
		oldSli := s.rules.slice
		s.rules.slice = true
		real.X = s.exprAsValue(real.X)
		for i := 0; i < len(real.Index); i++ {
			real.Index[i] = s.exprAsValue(real.Index[i])
		}
		s.rules.slice = oldSli

	case *AssertExpr:
		real.X = s.exprAsValue(real.X)
		real.Type = s.exprAsType(real.Type)

	case *TypeSwitchGuard:
		real.Lhs = s.exprAsValue(real.Lhs).(*Name)
		real.X = s.exprAsValue(real.X)

	case *Operation:
		real.X = s.exprAsValue(real.X)
		real.Y = s.exprAsValue(real.Y)

	case *CallExpr:
		real.Fun = s.expr(real.Fun)
		for i := 0; i < len(real.ArgList); i++ {
			real.ArgList[i] = s.exprAsValue(real.ArgList[i])
		}

	case *ListExpr:
		for i, e := range real.ElemList {
			real.ElemList[i] = s.exprAsValue(e)
		}

	case *ArrayType:
		real.Elem = s.exprAsType(real.Elem)
		real.Len = s.expr(real.Len)

	case *SliceType:
		real.Elem = s.exprAsType(real.Elem)

	case *DotsType:
		real.Elem = s.exprAsType(real.Elem)

	case *StructType:
		for _, f := range real.FieldList {
			f.Name = s.exprAsValue(f.Name).(*Name)
			f.Type = s.exprAsType(f.Type)
		}

	case *InterfaceType:
		for _, m := range real.MethodList {
			m.Name = s.exprAsValue(m.Name).(*Name)
			m.Type = s.exprAsType(m.Type)
		}

	case *FuncType:
		for _, parm := range real.ParamList {
			parm.Name = s.exprAsValue(parm.Name).(*Name)
			parm.Type = s.exprAsType(parm.Type)
		}

		for _, res := range real.ResultList {
			if res.Name != nil {
				res.Name = s.exprAsValue(res.Name).(*Name)
			}

			res.Type = s.exprAsType(res.Type)
		}

	case *MapType:
		real.Key = s.exprAsType(real.Key)
		real.Value = s.exprAsType(real.Value)

	case *ChanType:
		real.Elem = s.exprAsType(real.Elem)

	case *BadExpr:
		// impossible
	default:
		panic(fmt.Sprintf("unhandled expr %T", e))
	}
	return e
}

type scope struct {
	parent *scope
	names  map[string]*Name
}

func (s *scope) set(name string, node *Name) { s.names[name] = node }
func (s *scope) get(name string) *Name       { return s.names[name] }
func (s *scope) defined(name string) bool    { return s.get(name) != nil }
func (s *scope) available(name string) bool  { return s.find(name) != nil }

func (s *scope) find(name string) *Name {
	got := s.get(name)
	if got == nil && s.parent != nil {
		got = s.parent.find(name)
	}
	return got
}
