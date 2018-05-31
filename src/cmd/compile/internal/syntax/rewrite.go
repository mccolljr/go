package syntax

import (
	"fmt"
)

type rewriter struct {
	currentFIle    *File
	collectTargets []Expr
}

type bailout struct{}

func RewriteFile(f *File) (ret error) {
	r := &rewriter{}

	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("%T\n", err)
			if got, ok := err.(error); ok {
				ret = got
			} else {
				panic(err)
			}
		}
	}()

	r.file(f)

	return
}

func (r *rewriter) error(str string, pos Pos) {
	panic(Error{Pos: pos, Msg: str})
}

func (r *rewriter) pushCollectTarget(targ Expr) {
	var (
		_, isName = targ.(*Name)
		_, isSel  = targ.(*SelectorExpr)
	)
	// assert invariant
	if !(isName || isSel) {
		panic("internal: passed bad collect expression")
	}

	r.collectTargets = append(r.collectTargets, targ)
}

func (r *rewriter) popCollectTarget() Expr {
	var result Expr
	if result = r.getCollectionTarget(); result != nil {
		r.collectTargets = r.collectTargets[:len(r.collectTargets)-1]
	}
	return result
}

func (r *rewriter) getCollectionTarget() Expr {
	if len(r.collectTargets) == 0 {
		return nil
	}

	return r.collectTargets[len(r.collectTargets)-1]
}

func (r *rewriter) collectName() *Name {
	if n, ok := r.getCollectionTarget().(*Name); ok {
		return n
	}
	return nil
}

func (r *rewriter) file(f *File) {
	r.currentFIle = f
	for _, decl := range f.DeclList {
		r.decl(decl)
	}
}

func (r *rewriter) decl(d Decl) {
	switch n := d.(type) {
	case *FuncDecl:
		r.funcDecl(n)
	}
}

func (r *rewriter) stmtList(list []Stmt, label int) ([]Stmt, int) {
	for i := 0; i < len(list); i++ {
		stmt := list[i]
		var (
			replace   []Stmt
			labelNext bool
		)
		replace, label, labelNext = r.stmt(stmt, label)
		if labelNext {
			replace = append(replace, &LabeledStmt{
				Label: labelName(label),
				Stmt:  &EmptyStmt{},
			})
			label++
		}

		if len(replace) != 0 {
			pre, post := list[:i], list[i+1:]
			list = append(append(append([]Stmt{}, pre...), replace...), post...)
			i = i + len(replace) - 1
		}
	}

	return list, label
}

func (r *rewriter) funcDecl(f *FuncDecl) {
	if f.Body == nil {
		return
	}

	f.Body.List, _ = r.stmtList(f.Body.List, 1)
}

func (r *rewriter) stmt(in Stmt, label int) (
	replace []Stmt,
	nextLabel int,
	labelNext bool,
) {
	nextLabel = label
	switch s := in.(type) {
	case *CollectStmt:
		r.pushCollectTarget(s.Target)
		s.Body.List, nextLabel = r.stmtList(s.Body.List, label)
		r.popCollectTarget()
		replace = []Stmt{s.Body}
		labelNext = true

	case *BlockStmt:
		s.List, nextLabel = r.stmtList(s.List, label)

	case *AssignStmt:
		//fmt.Println("assignment")
		//fmt.Println("OP", s.Op.String())
		if s.Op == Def {
			// assert that no replacements would happen
			// (i.e. assert _! isn't an assignee)
			//fmt.Println("=== disallow defs")
			_, _ = r.lhs(s.Lhs, false)
		} else {
			didReplace := false
			s.Lhs, didReplace = r.lhs(s.Lhs, r.getCollectionTarget() != nil)
			if didReplace {
				replace = []Stmt{
					s,
					&IfStmt{
						stmt: stmt{node: node{pos: s.pos}},
						Cond: &Operation{
							expr: expr{node: node{pos: s.pos}},
							Op:   Neq,
							X:    r.getCollectionTarget(),
							Y:    &Name{Value: "nil"},
						},
						Then: &BlockStmt{
							stmt:   stmt{node: node{pos: s.pos}},
							Rbrace: s.pos,
							List: []Stmt{
								&BranchStmt{
									stmt:  stmt{node: node{pos: s.pos}},
									Tok:   Goto,
									Label: labelName(label),
								},
							},
						},
					},
				} // end replace block
			} // end did replace
		} // end else
	}

	return replace, nextLabel, labelNext
}

func (r *rewriter) collectStmt(s *CollectStmt, label int) (Stmt, int) {
	s.Body.List, label = r.stmtList(s.Body.List, label)
	return s.Body, label
}

func (r *rewriter) lhs(exp Expr, allowct bool) (got Expr, didReplace bool) {
	switch e := exp.(type) {
	case *Name:
		return r.assignee(e, allowct)
	case *ListExpr:
		for i, x := range e.ElemList {
			a, b := r.assignee(x, allowct)
			if b {
				didReplace = true
			}
			e.ElemList[i] = a
		}

		return e, didReplace
	}

	return exp, false
}

func (r *rewriter) assignee(exp Expr, allowct bool) (got Expr, didReplace bool) {
	//fmt.Printf("assignee: %T=%v\n", exp, exp)
	switch e := exp.(type) {
	case *Name:
		if e.Value == "_!" {
			//fmt.Printf("assignee: _!\n")
			if !allowct {
				//fmt.Printf("disallow: _!\n")
				r.error("illegal _! on left-hand", e.pos)
			}

			return r.getCollectionTarget(), true
		}
	}

	return exp, false
}

func labelName(i int) *Name { return &Name{Value: fmt.Sprintf("_collect_end_%d", i)} }
