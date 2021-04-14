package vile

import (
	"bytes"
)

// Key - the key type for Structs. The string value and Vile type string are combined, so we can extract
// the type later when enumerating keys. Map keys cannot Objects, they are not "comparable" in golang.
type structKey struct {
	keyValue string
	keyType  string
}

func newStructKey(v *Object) structKey {
	return structKey{v.text, v.Type.text}
}

func (k structKey) toObject() *Object {
	if k.keyType == "<string>" {
		return String(k.keyValue)
	}
	return Intern(k.keyValue)
}

// IsValidStructKey - return true of the object is a valid <struct> key.
func IsValidStructKey(o *Object) bool {
	switch o.Type {
	case StringType, SymbolType, KeywordType, TypeType:
		return true
	}
	return false
}

// EmptyStruct - a <struct> with no bindings
var EmptyStruct = MakeStruct(0)

// MakeStruct - create an empty <struct> object with the specified capacity
func MakeStruct(capacity int) *Object {
	strct := new(Object)
	strct.Type = StructType
	strct.bindings = make(map[structKey]*Object, capacity)
	return strct
}

// Struct - create a new <struct> object from the arguments, which can be other structs, or key/value pairs
func Struct(fieldvals []*Object) (*Object, error) {
	strct := new(Object)
	strct.Type = StructType
	strct.bindings = make(map[structKey]*Object)
	count := len(fieldvals)
	i := 0
	var bindings map[structKey]*Object
	for i < count {
		o := Value(fieldvals[i])
		i++
		switch o.Type {
		case StructType: // not a valid key, just copy bindings from it
			if bindings == nil {
				bindings = make(map[structKey]*Object, len(o.bindings))
			}
			for k, v := range o.bindings {
				bindings[k] = v
			}
		case StringType, SymbolType, KeywordType, TypeType:
			if i == count {
				return nil, Error(ArgumentErrorKey, "Mismatched keyword/value in arglist: ", o)
			}
			if bindings == nil {
				bindings = make(map[structKey]*Object)
			}
			bindings[newStructKey(o)] = fieldvals[i]
			i++
		default:
			return nil, Error(ArgumentErrorKey, "Bad struct key: ", o)
		}
	}
	if bindings == nil {
		strct.bindings = make(map[structKey]*Object)
	} else {
		strct.bindings = bindings
	}
	return strct, nil
}

// StructLength - return the length (field count) of the <struct> object
func StructLength(strct *Object) int {
	return len(strct.bindings)
}

// Get - return the value for the key of the object. The Value() function is first called to
// handle typed instances of <struct>.
// This is called by the VM, when a keyword is used as a function.
func Get(obj *Object, key *Object) (*Object, error) {
	s := Value(obj) // defined in data.go - Value
	if s.Type != StructType {
		return nil, Error(ArgumentErrorKey, "get expected a <struct> argument, got a ", obj.Type)
	}
	return structGet(s, key), nil
}

func structGet(s *Object, key *Object) *Object {
	switch key.Type {
	case KeywordType, SymbolType, TypeType, StringType:
		k := newStructKey(key)
		result, ok := s.bindings[k]
		if ok {
			return result
		}
	}
	return Null
}

func Has(obj *Object, key *Object) (bool, error) {
	tmp, err := Get(obj, key)
	if err != nil || tmp == Null {
		return false, err
	}
	return true, nil
}

func Put(obj *Object, key *Object, val *Object) {
	k := newStructKey(key)
	obj.bindings[k] = val
}

func Unput(obj *Object, key *Object) {
	k := newStructKey(key)
	delete(obj.bindings, k)
}

func sliceContains(slice []*Object, obj *Object) bool {
	for _, o := range slice {
		if o == obj {
			return true
		}
	}
	return false
}

func slicePut(bindings []*Object, key *Object, val *Object) []*Object {
	size := len(bindings)
	for i := 0; i < size; i += 2 {
		if key == bindings[i] {
			bindings[i+1] = val
			return bindings
		}
	}
	return append(append(bindings, key), val)
}

func validateKeywordArgList(args *Object, keys []*Object) (*Object, error) {
	tmp, err := validateKeywordArgBindings(args, keys)
	if err != nil {
		return nil, err
	}
	return ListFromValues(tmp), nil
}

func validateKeywordArgBindings(args *Object, keys []*Object) ([]*Object, error) {
	count := ListLength(args)
	bindings := make([]*Object, 0, count)
	for args != EmptyList {
		key := Car(args)
		switch key.Type {
		case SymbolType:
			key = Intern(key.text + ":")
			fallthrough
		case KeywordType:
			if !sliceContains(keys, key) {
				return nil, Error(ArgumentErrorKey, key, " bad keyword parameter. Allowed keys: ", keys)
			}
			args = args.cdr
			if args == EmptyList {
				return nil, Error(ArgumentErrorKey, key, " mismatched keyword/value pair in parameter")
			}
			bindings = slicePut(bindings, key, Car(args))
		case StructType:
			for k, v := range key.bindings {
				sym := Intern(k.keyValue)
				if sliceContains(keys, sym) {
					bindings = slicePut(bindings, sym, v)
				}
			}
		default:
			return nil, Error(ArgumentErrorKey, "Not a keyword: ", key)
		}
		args = args.cdr
	}
	return bindings, nil
}

// Equal returns true if the object is equal to the argument
func StructEqual(s1 *Object, s2 *Object) bool {
	bindings1 := s1.bindings
	size := len(bindings1)
	bindings2 := s2.bindings
	if size == len(bindings2) {
		for k, v := range bindings1 {
			v2, ok := bindings2[k]
			if !ok {
				return false
			}
			if !Equal(v, v2) {
				return false
			}
		}
		return true
	}
	return false
}

func structToString(s *Object) string {
	var buf bytes.Buffer
	buf.WriteString("{")
	first := true
	for k, v := range s.bindings {
		if first {
			first = false
		} else {
			buf.WriteString(" ")
		}
		buf.WriteString(k.keyValue)
		buf.WriteString(" ")
		buf.WriteString(v.String())
	}
	buf.WriteString("}")
	return buf.String()
}

func structToList(s *Object) (*Object, error) {
	result := EmptyList
	tail := EmptyList
	for k, v := range s.bindings {
		tmp := List(k.toObject(), v)
		if result == EmptyList {
			result = List(tmp)
			tail = result
		} else {
			tail.cdr = List(tmp)
			tail = tail.cdr
		}
	}
	return result, nil
}

func structToVector(s *Object) *Object {
	size := len(s.bindings)
	el := make([]*Object, size)
	j := 0
	for k, v := range s.bindings {
		el[j] = Vector(k.toObject(), v)
		j++
	}
	return VectorFromElements(el, size)
}

func StructKeys(s *Object) *Object {
	return structKeyList(s)
}

func StructValues(s *Object) *Object {
	return structValueList(s)
}

func structKeyList(s *Object) *Object {
	result := EmptyList
	tail := EmptyList
	for k := range s.bindings {
		key := k.toObject()
		if result == EmptyList {
			result = List(key)
			tail = result
		} else {
			tail.cdr = List(key)
			tail = tail.cdr
		}
	}
	return result
}

func structValueList(s *Object) *Object {
	result := EmptyList
	tail := EmptyList
	for _, v := range s.bindings {
		if result == EmptyList {
			result = List(v)
			tail = result
		} else {
			tail.cdr = List(v)
			tail = tail.cdr
		}
	}
	return result
}

func listToStruct(lst *Object) (*Object, error) {
	strct := new(Object)
	strct.Type = StructType
	strct.bindings = make(map[structKey]*Object)
	for lst != EmptyList {
		k := lst.car
		lst = lst.cdr
		switch k.Type {
		case ListType:
			if EmptyList == k || EmptyList == k.cdr || EmptyList != k.cdr.cdr {
				return nil, Error(ArgumentErrorKey, "Bad struct binding: ", k)
			}
			if !IsValidStructKey(k.car) {
				return nil, Error(ArgumentErrorKey, "Bad struct key: ", k.car)
			}
			Put(strct, k.car, k.cdr.car)
		case VectorType:
			elements := k.elements
			n := len(elements)
			if n != 2 {
				return nil, Error(ArgumentErrorKey, "Bad struct binding: ", k)
			}
			if !IsValidStructKey(elements[0]) {
				return nil, Error(ArgumentErrorKey, "Bad struct key: ", elements[0])
			}
			Put(strct, elements[0], elements[1])
		default:
			if !IsValidStructKey(k) {
				return nil, Error(ArgumentErrorKey, "Bad struct key: ", k)
			}
			if lst == EmptyList {
				return nil, Error(ArgumentErrorKey, "Mismatched keyword/value in list: ", k)
			}
			Put(strct, k, lst.car)
			lst = lst.cdr
		}
	}
	return strct, nil
}

func vectorToStruct(vec *Object) (*Object, error) {
	count := len(vec.elements)
	strct := new(Object)
	strct.Type = StructType
	strct.bindings = make(map[structKey]*Object, count)
	i := 0
	for i < count {
		k := vec.elements[i]
		i++
		switch k.Type {
		case ListType:
			if EmptyList == k || EmptyList == k.cdr || EmptyList != k.cdr.cdr {
				return nil, Error(ArgumentErrorKey, "Bad struct binding: ", k)
			}
			if !IsValidStructKey(k.car) {
				return nil, Error(ArgumentErrorKey, "Bad struct key: ", k.car)
			}
			Put(strct, k.car, k.cdr.car)
		case VectorType:
			elements := k.elements
			n := len(elements)
			if n != 2 {
				return nil, Error(ArgumentErrorKey, "Bad struct binding: ", k)
			}
			if !IsValidStructKey(elements[0]) {
				return nil, Error(ArgumentErrorKey, "Bad struct key: ", k.elements[0])
			}
			Put(strct, elements[0], elements[1])
		case StringType, SymbolType, KeywordType, TypeType:
		default:
			if !IsValidStructKey(k) {
				return nil, Error(ArgumentErrorKey, "Bad struct key: ", k)
			}
			if i == count {
				return nil, Error(ArgumentErrorKey, "Mismatched keyword/value in vector: ", k)
			}
			Put(strct, k, vec.elements[i])
			i++
		}
	}
	return strct, nil
}

func ToStruct(obj *Object) (*Object, error) {
	val := Value(obj)
	switch val.Type {
	case StructType:
		return val, nil
	case ListType:
		return listToStruct(val)
	case VectorType:
		return vectorToStruct(val)
	}
	return nil, Error(ArgumentErrorKey, "to-struct cannot accept argument of type ", obj.Type)
}
