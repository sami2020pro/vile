package vile

import (
	"bytes"
)

// VectorEqual - return true of the two vectors are equal, i.e. the same length and
// all the elements are also equal
func VectorEqual(v1 *Object, v2 *Object) bool {
	el1 := v1.elements
	el2 := v2.elements
	count := len(el1)
	if count != len(el2) {
		return false
	}
	for i := 0; i < count; i++ {
		if !Equal(el1[i], el2[i]) {
			return false
		}
	}
	return true
}

func vectorToString(vec *Object) string {
	el := vec.elements
	var buf bytes.Buffer
	buf.WriteString("[")
	count := len(el)
	if count > 0 {
		buf.WriteString(el[0].String())
		for i := 1; i < count; i++ {
			buf.WriteString(" ")
			buf.WriteString(el[i].String())
		}
	}
	buf.WriteString("]")
	return buf.String()
}

// MakeVector - create a new <vector> object of the specified size, with all elements initialized to
// the specified value
func MakeVector(size int, init *Object) *Object {
	elements := make([]*Object, size)
	for i := 0; i < size; i++ {
		elements[i] = init
	}
	return VectorFromElementsNoCopy(elements)
}

// Vector - create a new <vector> object from the given element objects.
func Vector(elements ...*Object) *Object {
	return VectorFromElements(elements, len(elements))
}

// VectorFromElements - return a new <vector> object from the given slice of elements. The slice is copied.
func VectorFromElements(elements []*Object, count int) *Object {
	el := make([]*Object, count)
	copy(el, elements[0:count])
	return VectorFromElementsNoCopy(el)
}

// VectorFromElementsNoCopy - create a new <vector> object from the given slice of elements. The slice is NOT copied.
func VectorFromElementsNoCopy(elements []*Object) *Object {
	vec := new(Object)
	vec.Type = VectorType
	vec.elements = elements
	return vec
}

// CopyVector - return a copy of the <vector>
func CopyVector(vec *Object) *Object {
	return VectorFromElements(vec.elements, len(vec.elements))
}

// ToVector - convert the object to a <vector>, if possible
func ToVector(obj *Object) (*Object, error) {
	switch obj.Type {
	case VectorType:
		return obj, nil
	case ListType:
		return listToVector(obj), nil
	case StructType:
		return structToVector(obj), nil
	case StringType:
		return stringToVector(obj), nil
	}
	return nil, Error(ArgumentErrorKey, "to-vector expected <vector>, <list>, <struct>, or <string>, got a ", obj.Type)
}
