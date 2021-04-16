package vile

import (
	"bytes"
)

/*
 * The good example of "rest" is JavaScript
 * JavaScript e.g
 *
   const sum = (...num) => {
       console.log(num.reduce((previous, current) => {
           return previous + current
       }))
   }

   sum(1, 2, 3, 4, 5)
 *
 */

// Cons - create a new list consisting of the first object and the rest of the list
func Cons(car *Object, cdr *Object) *Object {
	result := new(Object)
	result.Type = ListType
	result.car = car
	result.cdr = cdr
	return result
}

// Car - return the first object in a list
func Car(lst *Object) *Object {
	if lst == EmptyList {
		return Null
	}
	return lst.car
}

// Cdr - return the rest of the list
func Cdr(lst *Object) *Object {
	if lst == EmptyList {
		return lst
	}
	return lst.cdr
}

// Caar - return the Car of the Car of the list
func Caar(lst *Object) *Object {
	return Car(Car(lst))
}

// Cadr - return the Car of the Cdr of the list
func Cadr(lst *Object) *Object {
	return Car(Cdr(lst))
}

// Cdar - return the Cdr of the Car of the list
func Cdar(lst *Object) *Object {
	return Car(Cdr(lst))
}

// Cddr - return the Cdr of the Cdr of the list
func Cddr(lst *Object) *Object {
	return Cdr(Cdr(lst))
}

// Cadar - return the Car of the Cdr of the Car of the list
func Cadar(lst *Object) *Object {
	return Car(Cdr(Car(lst)))
}

// Caddr - return the Car of the Cdr of the Cdr of the list
func Caddr(lst *Object) *Object {
	return Car(Cdr(Cdr(lst)))
}

// Cdddr - return the Cdr of the Cdr of the Cdr of the list
func Cdddr(lst *Object) *Object {
	return Cdr(Cdr(Cdr(lst)))
}

// Cadddr - return the Car of the Cdr of the Cdr of the Cdr of the list
func Cadddr(lst *Object) *Object {
	return Car(Cdr(Cdr(Cdr(lst))))
}

// Cddddr - return the Cdr of the Cdr of the Cdr of the Cdr of the list
func Cddddr(lst *Object) *Object {
	return Cdr(Cdr(Cdr(Cdr(lst))))
}

var QuoteSymbol = Intern("quote")
var QuasiquoteSymbol = Intern("quasiquote")
var UnquoteSymbol = Intern("unquote")
var UnquoteSymbolSplicing = Intern("unquote-splicing")

// EmptyList - the value of (), terminates linked lists
var EmptyList = initEmpty()

func initEmpty() *Object {
	return &Object{Type: ListType} //car and cdr are both nil
}

// ListEqual returns true if the object is equal to the argument
func ListEqual(lst *Object, a *Object) bool {
	for lst != EmptyList {
		if a == EmptyList {
			return false
		}
		if !Equal(lst.car, a.car) {
			return false
		}
		lst = lst.cdr
		a = a.cdr
	}
	if lst == a {
		return true
	}
	return false
}

func listToString(lst *Object) string {
	var buf bytes.Buffer
	if lst != EmptyList && lst.cdr != EmptyList && Cddr(lst) == EmptyList {
		if lst.car == QuoteSymbol {
			buf.WriteString("'")
			buf.WriteString(Cadr(lst).String())
			return buf.String()
		} else if lst.car == QuasiquoteSymbol {
			buf.WriteString("`")
			buf.WriteString(Cadr(lst).String())
			return buf.String()
		} else if lst.car == UnquoteSymbol {
			buf.WriteString("~")
			buf.WriteString(Cadr(lst).String())
			return buf.String()
		} else if lst.car == UnquoteSymbolSplicing {
			buf.WriteString("~")
			buf.WriteString(Cadr(lst).String())
			return buf.String()
		}
	}
	buf.WriteString("(")
	delim := ""
	for lst != EmptyList {
		buf.WriteString(delim)
		delim = " "
		buf.WriteString(lst.car.String())
		lst = lst.cdr
	}
	buf.WriteString(")")
	return buf.String()
}

func ListLength(lst *Object) int {
	if lst == EmptyList {
		return 0
	}
	count := 1
	o := lst.cdr
	for o != EmptyList {
		count++
		o = o.cdr
	}
	return count
}

func MakeList(count int, val *Object) *Object {
	result := EmptyList
	for i := 0; i < count; i++ {
		result = Cons(val, result)
	}
	return result
}

func ListFromValues(values []*Object) *Object {
	p := EmptyList
	for i := len(values) - 1; i >= 0; i-- {
		v := values[i]
		p = Cons(v, p)
	}
	return p
}

func List(values ...*Object) *Object {
	return ListFromValues(values)
}

func listToVector(lst *Object) *Object {
	var elems []*Object
	for lst != EmptyList {
		elems = append(elems, lst.car)
		lst = lst.cdr
	}
	return VectorFromElementsNoCopy(elems)
}

// ToList - convert the argument to a List, if possible
func ToList(obj *Object) (*Object, error) {
	switch obj.Type {
	case ListType:
		return obj, nil
	case VectorType:
		return ListFromValues(obj.elements), nil
	case StructType:
		return structToList(obj)
	case StringType:
		return stringToList(obj), nil
	}
	return nil, Error(ArgumentErrorKey, "to-list cannot accept ", obj.Type)
}

func ReverseList(lst *Object) *Object {
	rev := EmptyList
	for lst != EmptyList {
		rev = Cons(lst.car, rev)
		lst = lst.cdr
	}
	return rev
}

func Flatten(lst *Object) *Object {
	result := EmptyList
	tail := EmptyList
	for lst != EmptyList {
		item := lst.car
		switch item.Type {
		case ListType:
			item = Flatten(item)
		case VectorType:
			litem, _ := ToList(item)
			item = Flatten(litem)
		default:
			item = List(item)
		}
		if tail == EmptyList {
			result = item
			tail = result
		} else {
			tail.cdr = item
		}
		for tail.cdr != EmptyList {
			tail = tail.cdr
		}
		lst = lst.cdr
	}
	return result
}

func Concat(seq1 *Object, seq2 *Object) (*Object, error) {
	rev := Reverse(seq1)
	if rev == EmptyList {
		return seq2, nil
	}
	lst := seq2
	for rev != EmptyList {
		lst = Cons(rev.car, lst)
		rev = rev.cdr
	}
	return lst, nil
}
