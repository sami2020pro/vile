package vile

// Compile - compile the source into a code object
func Compile(expr *Object) (*Object, error) {
	target := MakeCode(0, nil, nil, "")

	err := compileExpr(target, EmptyList, expr, false, false, "")

	if err != nil {
		return nil, err
	}
	target.code.emitReturn()
	return target, nil
}

func calculateLocation(sym *Object, env *Object) (int, int, bool) {
	i := 0
	for env != EmptyList {
		j := 0
		ee := Car(env)
		for ee != EmptyList {
			if Car(ee) == sym {
				return i, j, true
			}
			j++
			ee = Cdr(ee)
		}
		i++
		env = Cdr(env)
	}
	return -1, -1, false
}

func compileSelfEvalLiteral(target *Object, expr *Object, isTail bool, ignoreResult bool) error {
	if !ignoreResult {
		target.code.emitLiteral(expr)
		if isTail {
			target.code.emitReturn()
		}
	}
	return nil
}

func compileSymbol(target *Object, env *Object, expr *Object, isTail bool, ignoreResult bool) error {
	if GetMacro(expr) != nil {
		return Error(Intern("macro-error"), "Cannot use macro as a value: ", expr)
	}
	if i, j, ok := calculateLocation(expr, env); ok {
		target.code.emitLocal(i, j)
	} else {
		target.code.emitGlobal(expr)
	}
	if ignoreResult {
		target.code.emitPop()
	} else if isTail {
		target.code.emitReturn()
	}
	return nil
}

func compileQuote(target *Object, expr *Object, isTail bool, ignoreResult bool, lstlen int) error {
	if lstlen != 2 {
		return Error(SyntaxErrorKey, expr)
	}
	if !ignoreResult {
		target.code.emitLiteral(Cadr(expr))
		if isTail {
			target.code.emitReturn()
		}
	}
	return nil
}

func compileDef(target *Object, env *Object, lst *Object, isTail bool, ignoreResult bool, lstlen int) error {
	if lstlen < 3 {
		return Error(SyntaxErrorKey, lst)
	}
	sym := Cadr(lst)
	val := Caddr(lst)
	err := compileExpr(target, env, val, false, false, sym.String())
	if err == nil {
		target.code.emitDefGlobal(sym)
		if ignoreResult {
			target.code.emitPop()
		} else if isTail {
			target.code.emitReturn()
		}
	}
	return err
}

func compileUndef(target *Object, lst *Object, isTail bool, ignoreResult bool, lstlen int) error {
	if lstlen != 2 {
		return Error(SyntaxErrorKey, lst)
	}
	sym := Cadr(lst)
	if !IsSymbol(sym) {
		return Error(SyntaxErrorKey, lst)
	}
	target.code.emitUndefGlobal(sym)
	if ignoreResult {
	} else {
		target.code.emitLiteral(sym)
		if isTail {
			target.code.emitReturn()
		}
	}
	return nil
}

func compileMacro(target *Object, env *Object, expr *Object, isTail bool, ignoreResult bool, lstlen int) error {
	if lstlen != 3 {
		return Error(SyntaxErrorKey, expr)
	}
	var sym = Cadr(expr)
	if !IsSymbol(sym) {
		return Error(SyntaxErrorKey, expr)
	}
	err := compileExpr(target, env, Caddr(expr), false, false, sym.String())
	if err != nil {
		return err
	}
	if err == nil {
		target.code.emitDefMacro(sym)
		if ignoreResult {
			target.code.emitPop()
		} else if isTail {
			target.code.emitReturn()
		}
	}
	return err
}

func compileSet(target *Object, env *Object, lst *Object, isTail bool, ignoreResult bool, context string, lstlen int) error {
	if lstlen != 3 {
		return Error(SyntaxErrorKey, lst)
	}
	var sym = Cadr(lst)
	if !IsSymbol(sym) {
		return Error(SyntaxErrorKey, lst)
	}
	err := compileExpr(target, env, Caddr(lst), false, false, context)
	if err != nil {
		return err
	}
	if i, j, ok := calculateLocation(sym, env); ok {
		target.code.emitSetLocal(i, j)
	} else {
		target.code.emitDefGlobal(sym)
	}
	if ignoreResult {
		target.code.emitPop()
	} else if isTail {
		target.code.emitReturn()
	}
	return nil
}

func compileList(target *Object, env *Object, expr *Object, isTail bool, ignoreResult bool, context string) error {
	if expr == EmptyList {
		if !ignoreResult {
			target.code.emitLiteral(expr)
			if isTail {
				target.code.emitReturn()
			}
		}
		return nil
	}
	lst := expr
	lstlen := ListLength(lst)
	if lstlen == 0 {
		return Error(SyntaxErrorKey, lst)
	}
	fn := Car(lst)
	switch fn {
	case Intern("quote"):
		return compileQuote(target, expr, isTail, ignoreResult, lstlen)
	case Intern("do"):
		return compileSequence(target, env, Cdr(lst), isTail, ignoreResult, context)
	case Intern("if"):
		if lstlen == 3 || lstlen == 4 {
			return compileIfElse(target, env, Cadr(expr), Caddr(expr), Cdddr(expr), isTail, ignoreResult, context)
		}
		return Error(SyntaxErrorKey, expr)
	case Intern("var"):
		return compileDef(target, env, expr, isTail, ignoreResult, lstlen)
	case Intern("undef"):
		return compileUndef(target, expr, isTail, ignoreResult, lstlen)
	case Intern("macro"):
		return compileMacro(target, env, expr, isTail, ignoreResult, lstlen)
	case Intern("func"):
		if lstlen < 3 {
			return Error(SyntaxErrorKey, expr)
		}
		body := Cddr(lst)
		args := Cadr(lst)
		return compileFn(target, env, args, body, isTail, ignoreResult, context)
	case Intern("set!"):
		return compileSet(target, env, expr, isTail, ignoreResult, context, lstlen)
	case Intern("code"):
		return target.code.loadOps(Cdr(expr))
	case Intern("import"):
		return compileImport(target, Cdr(lst))
	default:
		fn, args := optimizeFuncall(fn, Cdr(lst))
		return compileFuncall(target, env, fn, args, isTail, ignoreResult, context)
	}
}

func compileVector(target *Object, env *Object, expr *Object, isTail bool, ignoreResult bool, context string) error {
	vlen := len(expr.elements)
	for i := vlen - 1; i >= 0; i-- {
		obj := expr.elements[i]
		err := compileExpr(target, env, obj, false, false, context)
		if err != nil {
			return err
		}
	}
	if !ignoreResult {
		target.code.emitVector(vlen)
		if isTail {
			target.code.emitReturn()
		}
	}
	return nil
}

func compileStruct(target *Object, env *Object, expr *Object, isTail bool, ignoreResult bool, context string) error {
	vlen := len(expr.bindings) * 2
	vals := make([]*Object, 0, vlen)
	for k, v := range expr.bindings {
		vals = append(vals, k.toObject())
		vals = append(vals, v)
	}
	for i := vlen - 1; i >= 0; i-- {
		obj := vals[i]
		err := compileExpr(target, env, obj, false, false, context)
		if err != nil {
			return err
		}
	}
	if !ignoreResult {
		target.code.emitStruct(vlen)
		if isTail {
			target.code.emitReturn()
		}
	}
	return nil
}

func compileExpr(target *Object, env *Object, expr *Object, isTail bool, ignoreResult bool, context string) error {
	if IsKeyword(expr) || IsType(expr) {
		return compileSelfEvalLiteral(target, expr, isTail, ignoreResult)
	} else if IsSymbol(expr) {
		return compileSymbol(target, env, expr, isTail, ignoreResult)
	} else if IsList(expr) {
		return compileList(target, env, expr, isTail, ignoreResult, context)
	} else if IsVector(expr) {
		return compileVector(target, env, expr, isTail, ignoreResult, context)
	} else if IsStruct(expr) {
		return compileStruct(target, env, expr, isTail, ignoreResult, context)
	}
	if !ignoreResult {
		target.code.emitLiteral(expr)
		if isTail {
			target.code.emitReturn()
		}
	}
	return nil
}

func compileFn(target *Object, env *Object, args *Object, body *Object, isTail bool, ignoreResult bool, context string) error {
	argc := 0
	var syms []*Object
	var defaults []*Object
	var keys []*Object
	tmp := args
	rest := false
	if !IsSymbol(args) {
		if IsVector(tmp) {
			tmp, _ = ToList(tmp)
		}
		for tmp != EmptyList {
			a := Car(tmp)
			if IsVector(a) {
				if Cdr(tmp) != EmptyList {
					return Error(SyntaxErrorKey, tmp)
				}
				defaults = make([]*Object, 0, len(a.elements))
				for _, sym := range a.elements {
					def := Null
					if IsList(sym) {
						def = Cadr(sym)
						sym = Car(sym)
					}
					if !IsSymbol(sym) {
						return Error(SyntaxErrorKey, tmp)
					}
					syms = append(syms, sym)
					defaults = append(defaults, def)
				}
				tmp = EmptyList
				break
			} else if IsStruct(a) {
				if Cdr(tmp) != EmptyList {
					return Error(SyntaxErrorKey, tmp)
				}
				slen := len(a.bindings)
				defaults = make([]*Object, 0, slen)
				keys = make([]*Object, 0, slen)
				for k, defValue := range a.bindings {
					sym := k.toObject()
					if IsList(sym) && Car(sym) == Intern("quote") && Cdr(sym) != EmptyList {
						sym = Cadr(sym)
					} else {
						var err error
						sym, err = unkeyworded(sym)
						if err != nil {
							return Error(SyntaxErrorKey, tmp)
						}
					}
					if !IsSymbol(sym) {
						return Error(SyntaxErrorKey, tmp)
					}
					syms = append(syms, sym)
					keys = append(keys, sym)
					defaults = append(defaults, defValue)
				}
				tmp = EmptyList
				break
			} else if !IsSymbol(a) {
				return Error(SyntaxErrorKey, tmp)
			}
			if a == Intern("&") {
				rest = true
			} else {
				if rest {
					syms = append(syms, a)
					defaults = make([]*Object, 0)
					tmp = EmptyList
					break
				}
				argc++
				syms = append(syms, a)
			}
			tmp = Cdr(tmp)
		}
	}
	if tmp != EmptyList {
		if IsSymbol(tmp) {
			syms = append(syms, tmp)
			defaults = make([]*Object, 0)
		} else {
			return Error(SyntaxErrorKey, tmp)
		}
	}
	args = ListFromValues(syms)
	newEnv := Cons(args, env)
	fnCode := MakeCode(argc, defaults, keys, context)
	err := compileSequence(fnCode, newEnv, body, true, false, context)
	if err == nil {
		if !ignoreResult {
			target.code.emitClosure(fnCode)
			if isTail {
				target.code.emitReturn()
			}
		}
	}
	return err
}

func compileSequence(target *Object, env *Object, exprs *Object, isTail bool, ignoreResult bool, context string) error {
	if exprs != EmptyList {
		for Cdr(exprs) != EmptyList {
			err := compileExpr(target, env, Car(exprs), false, true, context)
			if err != nil {
				return err
			}
			exprs = Cdr(exprs)
		}
		return compileExpr(target, env, Car(exprs), isTail, ignoreResult, context)
	}
	return Error(SyntaxErrorKey, Cons(Intern("do"), exprs))
}

func optimizeFuncall(fn *Object, args *Object) (*Object, *Object) {
	size := ListLength(args)
	if size == 2 {
		switch fn {
		case Intern("+"):
			if Equal(One, Car(args)) {
				return Intern("inc"), Cdr(args)
			} else if Equal(One, Cadr(args)) {
				return Intern("inc"), List(Car(args))
			}
		case Intern("-"):
			if Equal(One, Cadr(args)) {
				return Intern("dec"), List(Car(args))
			}
		}
	}
	return fn, args
}

func compileFuncall(target *Object, env *Object, fn *Object, args *Object, isTail bool, ignoreResult bool, context string) error {
	argc := ListLength(args)
	if argc < 0 {
		return Error(SyntaxErrorKey, Cons(fn, args))
	}
	err := compileArgs(target, env, args, context)
	if err != nil {
		return err
	}
	err = compileExpr(target, env, fn, false, false, context)
	if err != nil {
		return err
	}
	if isTail {
		target.code.emitTailCall(argc)
	} else {
		target.code.emitCall(argc)
		if ignoreResult {
			target.code.emitPop()
		}
	}
	return nil
}

func compileArgs(target *Object, env *Object, args *Object, context string) error {
	if args != EmptyList {
		err := compileArgs(target, env, Cdr(args), context)
		if err != nil {
			return err
		}
		return compileExpr(target, env, Car(args), false, false, context)
	}
	return nil
}

func compileIfElse(target *Object, env *Object, predicate *Object, Consequent *Object, antecedentOptional *Object, isTail bool, ignoreResult bool, context string) error {
	antecedent := Null
	if antecedentOptional != EmptyList {
		antecedent = Car(antecedentOptional)
	}
	err := compileExpr(target, env, predicate, false, false, context)
	if err != nil {
		return err
	}
	loc1 := target.code.emitJumpFalse(0)
	err = compileExpr(target, env, Consequent, isTail, ignoreResult, context)
	if err != nil {
		return err
	}
	loc2 := 0
	if !isTail {
		loc2 = target.code.emitJump(0)
	}
	target.code.setJumpLocation(loc1)
	err = compileExpr(target, env, antecedent, isTail, ignoreResult, context)
	if err == nil {
		if !isTail {
			target.code.setJumpLocation(loc2)
		}
	}
	return err
}

func compileImport(target *Object, rest *Object) error {
	lstlen := ListLength(rest)
	if lstlen != 1 {
		return Error(SyntaxErrorKey, Cons(Intern("import"), rest))
	}
	sym := Car(rest)
	if !IsSymbol(sym) {
		return Error(SyntaxErrorKey, rest)
	}
	target.code.emitImport(sym)
	return nil
}
