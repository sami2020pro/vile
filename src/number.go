package vile

import (
	"math"
	"math/rand"
	"strconv"
)

// TEST
type Test struct {
	Value float64
}

func Integer(i int) *Test {
	return &Test{Value: float64(i)}
}

// Zero is the Vile 0 value
var Zero = Number(0)

// One is the Vile 1 value
var One = Number(1)

// MinusOne is the Vile -1 value
var MinusOne = Number(-1)

// Number - create a Number object for the given value
func Number(f float64) *Object {
	num := new(Object)
	num.Type = NumberType
	num.fval = f
	return num
}

func Int(n int64) *Object {
	return Number(float64(n))
}

// Round - return the closest integer value to the float value
func Round(f float64) float64 {
	if f > 0 {
		return math.Floor(f + 0.5)
	}
	return math.Ceil(f - 0.5)
}

// ToNumber - convert object to a number, if possible
func ToNumber(o *Object) (*Object, error) {
	switch o.Type {
	case NumberType:
		return o, nil
	case CharacterType:
		return Number(o.fval), nil
	case BooleanType:
		return Number(o.fval), nil
	case StringType:
		f, err := strconv.ParseFloat(o.text, 64)
		if err == nil {
			return Number(f), nil
		}
	}
	return nil, Error(ArgumentErrorKey, "cannot convert to an number: ", o)
}

// ToInt - convert the object to an integer number, if possible
func ToInt(o *Object) (*Object, error) {
	switch o.Type {
	case NumberType:
		return Number(Round(o.fval)), nil
	case CharacterType:
		return Number(o.fval), nil
	case BooleanType:
		return Number(o.fval), nil
	case StringType:
		n, err := strconv.ParseInt(o.text, 10, 64)
		if err == nil {
			return Number(float64(n)), nil
		}
	}
	return nil, Error(ArgumentErrorKey, "cannot convert to an integer: ", o)
}

func IsInt(obj *Object) bool {
	if obj.Type == NumberType {
		f := obj.fval
		if math.Trunc(f) == f {
			return true
		}
	}
	return false
}

func IsFloat(obj *Object) bool {
	if obj.Type == NumberType {
		return !IsInt(obj)
	}
	return false
}

func AsFloat64Value(obj *Object) (float64, error) {
	if obj.Type == NumberType {
		return obj.fval, nil
	}
	return 0, Error(ArgumentErrorKey, "Expected a <number>, got a ", obj.Type)
}

func AsInt64Value(obj *Object) (int64, error) {
	if obj.Type == NumberType {
		return int64(obj.fval), nil
	}
	return 0, Error(ArgumentErrorKey, "Expected a <number>, got a ", obj.Type)
}

func AsIntValue(obj *Object) (int, error) {
	if obj.Type == NumberType {
		return int(obj.fval), nil
	}
	return 0, Error(ArgumentErrorKey, "Expected a <number>, got a ", obj.Type)
}

func AsByteValue(obj *Object) (byte, error) {
	if obj.Type == NumberType {
		return byte(obj.fval), nil
	}
	return 0, Error(ArgumentErrorKey, "Expected a <number>, got a ", obj.Type)
}

const epsilon = 0.000000001

// Equal returns true if the object is equal to the argument, within epsilon
func NumberEqual(f1 float64, f2 float64) bool {
	if f1 == f2 {
		return true
	}
	if math.Abs(f1-f2) < epsilon {
		return true
	}
	return false
}

var randomGenerator = rand.New(rand.NewSource(1))

func RandomSeed(n int64) {
	randomGenerator = rand.New(rand.NewSource(n))
}

func Random(min float64, max float64) *Object {
	return Number(min + (randomGenerator.Float64() * (max - min)))
}

func RandomList(size int, min float64, max float64) *Object {
	result := EmptyList
	tail := EmptyList
	for i := 0; i < size; i++ {
		tmp := List(Random(min, max))
		if result == EmptyList {
			result = tmp
			tail = tmp
		} else {
			tail.cdr = tmp
			tail = tmp
		}
	}
	return result
}
