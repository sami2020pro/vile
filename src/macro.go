package vile

import (
	"fmt"
)

type macro struct {
	name     *Object
	expander *Object // a function of one argument
}

// Macro - create a new Macro
func Macro(name *Object, expander *Object) *macro {
	return &macro{name, expander}
}

func (mac *macro) String() string {
	return fmt.Sprintf("(macro %v %v)", mac.name, mac.expander)
}

// Macroexpand - return the expansion of all macros in the object and return the result
func Macroexpand(expr *Object) (*Object, error) {
	return macroexpandObject(expr)
}

func macroexpandObject(expr *Object) (*Object, error) {
	if IsList(expr) {
		if expr != EmptyList {
			return macroexpandList(expr)
		}
	}
	// check point println(expr)
	return expr, nil
}

func macroexpandList(expr *Object) (*Object, error) {
	if expr == EmptyList {
		return expr, nil
	}
	lst := expr
	fn := Car(lst)
	head := fn
	if IsSymbol(fn) {
		result, err := expandPrimitive(fn, lst)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
		head = fn
	} else if IsList(fn) {
		expanded, err := macroexpandList(fn)
		if err != nil {
			return nil, err
		}
		head = expanded
	}
	tail, err := expandSequence(Cdr(expr))
	if err != nil {
		return nil, err
	}
	return Cons(head, tail), nil
}

func (mac *macro) expand(expr *Object) (*Object, error) {
	expander := mac.expander
	if expander.Type == FunctionType {
		if expander.code != nil {
			if expander.code.argc == 1 {
				expanded, err := execCompileTime(expander.code, expr)
				if err == nil {
					if IsList(expanded) {
						return macroexpandObject(expanded)
					}
					return expanded, err
				}
				return nil, err
			}
		} else if expander.primitive != nil {
			args := []*Object{expr}
			expanded, err := expander.primitive.fun(args)
			if err == nil {
				return macroexpandObject(expanded)
			}
			return nil, err
		}
	}
	return nil, Error(MacroErrorKey, "Bad macro expander function: ", expander)
}

func expandSequence(seq *Object) (*Object, error) {
	var result []*Object
	if seq == nil {
		panic("Whoops: should be (), not nil!")
	}
	for seq != EmptyList {
		item := Car(seq)
		if IsList(item) {
			expanded, err := macroexpandList(item)
			if err != nil {
				return nil, err
			}
			result = append(result, expanded)
		} else {
			result = append(result, item)
		}
		seq = Cdr(seq)
	}
	lst := ListFromValues(result)
	if seq != EmptyList {
		tmp := Cons(seq, EmptyList)
		return Concat(lst, tmp)
	}
	return lst, nil
}

func expandIf(expr *Object) (*Object, error) {
	i := ListLength(expr)
	if i == 4 {
		tmp, err := expandSequence(Cdr(expr))
		if err != nil {
			return nil, err
		}
		return Cons(Car(expr), tmp), nil
	} else if i == 3 {
		tmp := List(Cadr(expr), Caddr(expr), Null)
		tmp, err := expandSequence(tmp)
		if err != nil {
			return nil, err
		}
		return Cons(Car(expr), tmp), nil
	} else {
		return nil, Error(SyntaxErrorKey, expr)
	}
}

func expandUndef(expr *Object) (*Object, error) {
	if ListLength(expr) != 2 || !IsSymbol(Cadr(expr)) {
		return nil, Error(SyntaxErrorKey, expr)
	}
	return expr, nil
}

func expandDefn(expr *Object) (*Object, error) {
	exprLen := ListLength(expr)
	if exprLen >= 4 {
		name := Cadr(expr)
		if IsSymbol(name) {
			args := Caddr(expr)
			body, err := expandSequence(Cdddr(expr))
			if err != nil {
				return nil, err
			}
			tmp, err := expandFn(Cons(Intern("func"), Cons(args, body)))
			if err != nil {
				return nil, err
			}
			return List(Intern("var"), name, tmp), nil
		}
	}

	return nil, Error(SyntaxErrorKey, expr)
}

func expandDefmacro(expr *Object) (*Object, error) {
	exprLen := ListLength(expr)
	if exprLen >= 4 {
		name := Cadr(expr)
		if IsSymbol(name) {
			args := Caddr(expr)
			body, err := expandSequence(Cdddr(expr))
			if err != nil {
				return nil, err
			}
			tmp, err := expandFn(Cons(Intern("func"), Cons(args, body)))
			if err != nil {
				return nil, err
			}
			sym := Intern("expr")
			tmp, err = expandFn(List(Intern("func"), List(sym), List(Intern("apply"), tmp, List(Intern("cdr"), sym))))
			if err != nil {
				return nil, err
			}
			return List(Intern("macro"), name, tmp), nil
		}
	}
	return nil, Error(SyntaxErrorKey, expr)
}

func expandDef(expr *Object) (*Object, error) {
	exprLen := ListLength(expr)
	if exprLen != 3 {
		return nil, Error(SyntaxErrorKey, expr)
	}
	name := Cadr(expr)
	if !IsSymbol(name) {
		return nil, Error(SyntaxErrorKey, expr)
	}
	if exprLen > 3 {
		return nil, Error(SyntaxErrorKey, expr)
	}
	body := Caddr(expr)
	if !IsList(body) {
		return expr, nil
	}
	val, err := macroexpandList(body)
	if err != nil {
		return nil, err
	}
	return List(Car(expr), name, val), nil
}

func expandFn(expr *Object) (*Object, error) {
	exprLen := ListLength(expr)
	if exprLen < 3 {
		return nil, Error(SyntaxErrorKey, expr)
	}
	body, err := expandSequence(Cddr(expr))
	if err != nil {
		return nil, err
	}
	bodyLen := ListLength(body)
	if bodyLen > 0 {
		tmp := body
		if IsList(tmp) && Caar(tmp) == Intern("var") || Caar(tmp) == Intern("macro") {
			bindings := EmptyList
			for Caar(tmp) == Intern("var") || Caar(tmp) == Intern("macro") {
				if Caar(tmp) == Intern("macro") {
					return nil, Error(MacroErrorKey, "macros can only be defined at top level")
				}
				def, err := expandDef(Car(tmp))
				if err != nil {
					return nil, err
				}
				bindings = Cons(Cdr(def), bindings)
				tmp = Cdr(tmp)
			}
			bindings = ReverseList(bindings)
			tmp = Cons(Intern("letrec"), Cons(bindings, tmp))
			tmp2, err := macroexpandList(tmp)
			return List(Car(expr), Cadr(expr), tmp2), err
		}
	}
	args := Cadr(expr)
	return Cons(Car(expr), Cons(args, body)), nil
}

func expandSetBang(expr *Object) (*Object, error) {
	exprLen := ListLength(expr)
	if exprLen != 3 {
		return nil, Error(SyntaxErrorKey, expr)
	}
	var val = Caddr(expr)
	if IsList(val) {
		v, err := macroexpandList(val)
		if err != nil {
			return nil, err
		}
		val = v
	}
	return List(Car(expr), Cadr(expr), val), nil
}

func expandPrimitive(fn *Object, expr *Object) (*Object, error) {
	switch fn {
	case Intern("quote"):
		return expr, nil
	case Intern("do"):
		return expandSequence(expr)
	case Intern("if"):
		return expandIf(expr)
	case Intern("var"):
		return expandDef(expr)
	case Intern("undef"):
		return expandUndef(expr)
	case Intern("fn"):
		return expandDefn(expr)
	case Intern("macro"):
		return expandDefmacro(expr)
	case Intern("func"):
		return expandFn(expr)
	case Intern("set!"):
		return expandSetBang(expr)
	case Intern("lap"):
		return expr, nil
	case Intern("import"):
		return expr, nil
	default:
		macro := GetMacro(fn)
		if macro != nil {
			tmp, err := macro.expand(expr)
			return tmp, err
		}
		return nil, nil
	}
}

func crackLetrecBindings(bindings *Object, tail *Object) (*Object, *Object, bool) {
	var names []*Object
	inits := EmptyList
	for bindings != EmptyList {
		if IsList(bindings) {
			tmp := Car(bindings)
			if IsList(tmp) {
				name := Car(tmp)
				if IsSymbol(name) {
					names = append(names, name)
				} else {
					return nil, nil, false
				}
				if IsList(Cdr(tmp)) {
					inits = Cons(Cons(Intern("set!"), tmp), inits)
				} else {
					return nil, nil, false
				}
			} else {
				return nil, nil, false
			}

		} else {
			return nil, nil, false
		}
		bindings = Cdr(bindings)
	}
	inits = ReverseList(inits)
	head := inits
	for inits.cdr != EmptyList {
		inits = inits.cdr
	}
	inits.cdr = tail
	return ListFromValues(names), head, true
}

func expandLetrec(expr *Object) (*Object, error) {
	body := Cddr(expr)
	if body == EmptyList {
		return nil, Error(SyntaxErrorKey, expr)
	}
	bindings := Cadr(expr)
	if !IsList(bindings) {
		return nil, Error(SyntaxErrorKey, expr)
	}
	names, body, ok := crackLetrecBindings(bindings, body)
	if !ok {
		return nil, Error(SyntaxErrorKey, expr)
	}
	code, err := macroexpandList(Cons(Intern("fn"), Cons(names, body)))
	if err != nil {
		return nil, err
	}
	values := MakeList(ListLength(names), Null)
	return Cons(code, values), nil
}

func crackLetBindings(bindings *Object) (*Object, *Object, bool) {
	var names []*Object
	var values []*Object
	for bindings != EmptyList {
		tmp := Car(bindings)
		if IsList(tmp) {
			name := Car(tmp)
			if IsSymbol(name) {
				names = append(names, name)
				tmp2 := Cdr(tmp)
				if tmp2 != EmptyList {
					val, err := macroexpandObject(Car(tmp2))
					if err == nil {
						values = append(values, val)
						bindings = Cdr(bindings)
						continue
					}
				}
			}
		}
		return nil, nil, false
	}
	return ListFromValues(names), ListFromValues(values), true
}

func expandLet(expr *Object) (*Object, error) {
	if IsSymbol(Cadr(expr)) {
		return expandNamedLet(expr)
	}
	bindings := Cadr(expr)
	if !IsList(bindings) {
		return nil, Error(SyntaxErrorKey, expr)
	}
	names, values, ok := crackLetBindings(bindings)
	if !ok {
		return nil, Error(SyntaxErrorKey, expr)
	}
	body := Cddr(expr)
	if body == EmptyList {
		return nil, Error(SyntaxErrorKey, expr)
	}
	code, err := macroexpandList(Cons(Intern("fn"), Cons(names, body)))
	if err != nil {
		return nil, err
	}
	return Cons(code, values), nil
}

func expandNamedLet(expr *Object) (*Object, error) {
	name := Cadr(expr)
	bindings := Caddr(expr)
	if !IsList(bindings) {
		return nil, Error(SyntaxErrorKey, expr)
	}
	names, values, ok := crackLetBindings(bindings)
	if !ok {
		return nil, Error(SyntaxErrorKey, expr)
	}
	body := Cdddr(expr)
	tmp := List(Intern("letrec"), List(List(name, Cons(Intern("func"), Cons(names, body)))), Cons(name, values))
	return macroexpandList(tmp)
}

func nextCondClause(expr *Object, clauses *Object, count int) (*Object, error) {
	var result *Object
	var err error
	tmpsym := Intern("__tmp__")
	ifsym := Intern("if")
	elsesym := Intern("else")
	letsym := Intern("let")
	dosym := Intern("do")

	clause0 := Car(clauses)
	next := Cdr(clauses)
	clause1 := Car(next)

	if count == 2 {
		if !IsList(clause1) {
			return nil, Error(SyntaxErrorKey, expr)
		}
		if elsesym == Car(clause1) {
			if Cadr(clause0) == Intern("=>") {
				if ListLength(clause0) != 3 {
					return nil, Error(SyntaxErrorKey, expr)
				}
				result = List(letsym, List(List(tmpsym, Car(clause0))), List(ifsym, tmpsym, List(Caddr(clause0), tmpsym), Cons(dosym, Cdr(clause1))))
			} else {
				result = List(ifsym, Car(clause0), Cons(dosym, Cdr(clause0)), Cons(dosym, Cdr(clause1)))
			}
		} else {
			if Cadr(clause1) == Intern("=>") {
				if ListLength(clause1) != 3 {
					return nil, Error(SyntaxErrorKey, expr)
				}
				result = List(letsym, List(List(tmpsym, Car(clause1))), List(ifsym, tmpsym, List(Caddr(clause1), tmpsym), clause1))
			} else {
				result = List(ifsym, Car(clause1), Cons(dosym, Cdr(clause1)))
			}
			if Cadr(clause0) == Intern("=>") {
				if ListLength(clause0) != 3 {
					return nil, Error(SyntaxErrorKey, expr)
				}
				result = List(letsym, List(List(tmpsym, Car(clause0))), List(ifsym, tmpsym, List(Caddr(clause0), tmpsym), result))
			} else {
				result = List(ifsym, Car(clause0), Cons(dosym, Cdr(clause0)), result)
			}
		}
	} else {
		result, err = nextCondClause(expr, next, count-1)
		if err != nil {
			return nil, err
		}
		if Cadr(clause0) == Intern("=>") {
			if ListLength(clause0) != 3 {
				return nil, Error(SyntaxErrorKey, expr)
			}
			result = List(letsym, List(List(tmpsym, Car(clause0))), List(ifsym, tmpsym, List(Caddr(clause0), tmpsym), result))
		} else {
			result = List(ifsym, Car(clause0), Cons(dosym, Cdr(clause0)), result)
		}
	}
	return macroexpandObject(result)
}

func expandCond(expr *Object) (*Object, error) {
	i := ListLength(expr)
	if i < 2 {
		return nil, Error(SyntaxErrorKey, expr)
	} else if i == 2 {
		tmp := Cadr(expr)
		if Car(tmp) == Intern("else") {
			tmp = Cons(Intern("do"), Cdr(tmp))
		} else {
			expr = Cons(Intern("do"), Cdr(tmp))
			tmp = List(Intern("if"), Car(tmp), expr)
		}
		return macroexpandObject(tmp)
	} else {
		return nextCondClause(expr, Cdr(expr), i-1)
	}
}

func expandQuasiquote(expr *Object) (*Object, error) {
	if ListLength(expr) != 2 {
		return nil, Error(SyntaxErrorKey, expr)
	}
	return expandQQ(Cadr(expr))
}

func expandQQ(expr *Object) (*Object, error) {
	switch expr.Type {
	case ListType:
		if expr == EmptyList {
			return expr, nil
		}
		if expr.cdr != EmptyList {
			if expr.car == UnquoteSymbol {
				if expr.cdr.cdr != EmptyList {
					return nil, Error(SyntaxErrorKey, expr)
				}
				return macroexpandObject(expr.cdr.car)
			} else if expr.car == UnquoteSymbolSplicing {
				return nil, Error(MacroErrorKey, "unquote-splicing can only occur in the context of a list ")
			}
		}
		tmp, err := expandQQList(expr)
		if err != nil {
			return nil, err
		}
		return macroexpandObject(tmp)
	case SymbolType:
		return List(Intern("quote"), expr), nil
	default:
		return expr, nil
	}
}

func expandQQList(lst *Object) (*Object, error) {
	var tmp *Object
	var err error
	result := List(Intern("concat"))
	tail := result
	for lst != EmptyList {
		item := Car(lst)
		if IsList(item) && item != EmptyList {
			if Car(item) == QuasiquoteSymbol {
				return nil, Error(MacroErrorKey, "nested quasiquote not supported")
			}
			if Car(item) == UnquoteSymbol && ListLength(item) == 2 {
				tmp, err = macroexpandObject(Cadr(item))
				tmp = List(Intern("list"), tmp)
				if err != nil {
					return nil, err
				}
				tail.cdr = List(tmp)
				tail = tail.cdr
			} else if Car(item) == UnquoteSymbolSplicing && ListLength(item) == 2 {
				tmp, err = macroexpandObject(Cadr(item))
				if err != nil {
					return nil, err
				}
				tail.cdr = List(tmp)
				tail = tail.cdr
			} else {
				tmp, err = expandQQList(item)
				if err != nil {
					return nil, err
				}
				tail.cdr = List(List(Intern("list"), tmp))
				tail = tail.cdr
			}
		} else {
			tail.cdr = List(List(Intern("quote"), List(item)))
			tail = tail.cdr
		}
		lst = Cdr(lst)
	}
	return result, nil
}
