package vile

import (
	"bytes"
	"fmt"
	"strconv"
)

// Object is the Vile object: a union of all possible primitive types. Which fields are used depends on the variant
// the variant is a type object i.e. Intern("<string>"). For arbitrary embedded extension types, the Value field
// is an interface{}. It is used for Channels internally, but is generally useful for app-specific native types
// when extending Vile.
type Object struct {
	Type         *Object               // i.e. <string>
	code         *Code                 // non-nil for closure, code
	frame        *frame                // non-nil for closure, continuation
	primitive    *primitive            // non-nil for primitives
	continuation *continuation         // non-nil for continuation
	car          *Object               // non-nil for instances and lists
	cdr          *Object               // non-nil for slists, nil for everything else
	bindings     map[structKey]*Object // non-nil for struct
	elements     []*Object             // non-nil for vector
	fval         float64               // number
	text         string                // string, symbol, keyword, type
	Value        interface{}           // the rest of the data for more complex things
}

/* Go - continuation e.g
 *
 func factorial(n int, f func(int)) {
   if n == 1 {
       f(1) // base-case
   } else {
       factorial(n - 1, func(y int) { f(n * y) })
   }
 }
 */

// BoolValue - return native bool value of the object
func BoolValue(obj *Object) bool {
	if obj == True {
		return true
	}
	return false
}

// RuneValue - return native rune value of the object
func RuneValue(obj *Object) rune {
	return rune(obj.fval)
}

// IntValue - return native int value of the object
func IntValue(obj *Object) int {
	return int(obj.fval)
}

// Int64Value - return native int64 value of the object
func Int64Value(obj *Object) int64 {
	return int64(obj.fval)
}

// Float64Value - return native float64 value of the object
func Float64Value(obj *Object) float64 {
	return obj.fval
}

// StringValue - return native string value of the object
func StringValue(obj *Object) string {
	return obj.text
}

// NewObject is the constructor for externally defined objects, where the
// value is an interface{}.
func NewObject(variant *Object, value interface{}) *Object {
	lob := new(Object)
	lob.Type = variant
	lob.Value = value
	return lob
}

// Identical - return if two objects are identical
func Identical(o1 *Object, o2 *Object) bool {
	return o1 == o2
}

type stringable interface {
	String() string
}

func (lob *Object) String() string {
	switch lob.Type {
	case NullType:
		return "null"
	case BooleanType:
		if lob == True {
			return "true"
		}
		return "false"
	case CharacterType:
		return string([]rune{rune(lob.fval)})
	case NumberType:
		return strconv.FormatFloat(lob.fval, 'f', -1, 64)
	case StringType, SymbolType, KeywordType, TypeType:
		return lob.text
	case ListType:
		return listToString(lob)
	case VectorType:
		return vectorToString(lob)
	case StructType:
		return structToString(lob)
	case FunctionType:
		return functionToString(lob)
	case CodeType:
		return lob.code.String()
	case ErrorType:
		return "#<error>" + Write(lob.car)
	default:
		if lob.Value != nil {
			if s, ok := lob.Value.(stringable); ok {
				return s.String()
			}
			return "#[" + typeNameString(lob.Type.text) + "]"
		}
		return "#" + lob.Type.text + Write(lob.car)
	}
}

// TypeType is the metatype, the type of all types | Wikipedia => "... kind (a type of a type, or metatype) ..."
var TypeType *Object // bootstrapped in initSymbolTable => Intern("<type>")

// KeywordType is the type of all keywords
var KeywordType *Object // bootstrapped in initSymbolTable => Intern("<keyword>")

// SymbolType is the type of all symbols
var SymbolType *Object // bootstrapped in initSymbolTable = Intern("<symbol>")

// NullType the type of the null object
var NullType = Intern("<null>")

// BooleanType is the type of true and false
var BooleanType = Intern("<boolean>")

// CharacterType is the type of all characters
var CharacterType = Intern("<character>")

// NumberType is the type of all numbers
var NumberType = Intern("<number>")

// StringType is the type of all strings
var StringType = Intern("<string>")

// ListType is the type of all lists
var ListType = Intern("<list>")

// VectorType is the type of all vectors
var VectorType = Intern("<vector>")

// VectorType is the type of all structs
var StructType = Intern("<struct>")

// FunctionType is the type of all functions
var FunctionType = Intern("<function>")

// CodeType is the type of compiled code
var CodeType = Intern("<code>")

// ErrorType is the type of all errors
var ErrorType = Intern("<error>")

// AnyType is a pseudo type specifier indicating any type
var AnyType = Intern("<any>")

// Null is Vile's version of nil. It means "nothing" and is not the same as EmptyList. It is a singleton.
var Null = &Object{Type: NullType}

func IsNull(obj *Object) bool {
	return obj == Null
}

// True is the singleton boolean true value
var True = &Object{Type: BooleanType, fval: 1}

// False is the singleton boolean false value
var False = &Object{Type: BooleanType, fval: 0}

func IsBoolean(obj *Object) bool {
	return obj.Type == BooleanType
}

func IsCharacter(obj *Object) bool {
	return obj.Type == CharacterType
}
func IsNumber(obj *Object) bool {
	return obj.Type == NumberType
}
func IsString(obj *Object) bool {
	return obj.Type == StringType
}
func IsList(obj *Object) bool {
	return obj.Type == ListType
}
func IsVector(obj *Object) bool {
	return obj.Type == VectorType
}
func IsStruct(obj *Object) bool {
	return obj.Type == StructType
}
func IsFunction(obj *Object) bool {
	return obj.Type == FunctionType
}
func IsCode(obj *Object) bool {
	return obj.Type == CodeType
}
func IsSymbol(obj *Object) bool {
	return obj.Type == SymbolType
}
func IsKeyword(obj *Object) bool {
	return obj.Type == KeywordType
}
func IsType(obj *Object) bool {
	return obj.Type == TypeType
}

// instances have arbitrary Type symbols, all we can check is that the instanceValue is set
func IsInstance(obj *Object) bool {
	return obj.car != nil && obj.cdr == nil
}

func Equal(o1 *Object, o2 *Object) bool {
	if o1 == o2 {
		return true
	}
	if o1.Type != o2.Type {
		return false
	}
	switch o1.Type {
	case BooleanType, CharacterType:
		return int(o1.fval) == int(o2.fval)
	case NumberType:
		return NumberEqual(o1.fval, o2.fval)
	case StringType:
		return o1.text == o2.text
	case ListType:
		return ListEqual(o1, o2)
	case VectorType:
		return VectorEqual(o1, o2)
	case StructType:
		return StructEqual(o1, o2)
	case SymbolType, KeywordType, TypeType:
		return o1 == o2
	case NullType:
		return true
	default:
		o1a := Value(o1)
		if o1a != o1 {
			o2a := Value(o2)
			return Equal(o1a, o2a)
		}
		return false
	}
}

func IsPrimitiveType(tag *Object) bool {
	switch tag {
	case NullType, BooleanType, CharacterType, NumberType, StringType, ListType, VectorType, StructType:
		return true
	case SymbolType, KeywordType, TypeType, FunctionType:
		return true
	default:
		return false
	}
}

func Instance(tag *Object, val *Object) (*Object, error) {
	if !IsType(tag) {
		return nil, Error(ArgumentErrorKey, TypeType.text, tag)
	}
	if IsPrimitiveType(tag) {
		return val, nil
	}
	result := new(Object)
	result.Type = tag
	result.car = val
	return result, nil
}

func Value(obj *Object) *Object {
	if obj.cdr == nil && obj.car != nil {
		return obj.car
	}

	return obj
}

//
// Error - creates a new Error from the arguments. The first is an actual Vile keyword object,
// the rest are interpreted as/converted to strings
//
func Error(errkey *Object, args ...interface{}) error {
	var buf bytes.Buffer
	for _, o := range args {
		if l, ok := o.(*Object); ok {
			buf.WriteString(fmt.Sprintf("%v", Write(l)))
		} else {
			buf.WriteString(fmt.Sprintf("%v", o))
		}
	}
	if errkey.Type != KeywordType {
		errkey = ErrorKey
	}
	return MakeError(errkey, String(buf.String()))
}

func MakeError(elements ...*Object) *Object {
	data := Vector(elements...)
	return &Object{Type: ErrorType, car: data}
}

func theError(o interface{}) (*Object, bool) {
	if o == nil {
		return nil, false
	}
	if err, ok := o.(*Object); ok {
		if err.Type == ErrorType {
			return err, true
		}
	}
	return nil, false

}

func IsError(o interface{}) bool {
	_, ok := theError(o)
	return ok
}

func ErrorData(err *Object) *Object {
	return err.car
}

// Error
func (lob *Object) Error() string {
	if lob.Type == ErrorType {
		s := lob.car.String()
		if lob.text != "" {
			s += " [in " + lob.text + "]"
		}
		return s
	}
	return lob.String()
}

// ErrorKey - used to generic errors
var ErrorKey = Intern("error:")

// ArgumentErrorKey
var ArgumentErrorKey = Intern("argument-error:")

// SyntaxErrorKey
var SyntaxErrorKey = Intern("syntax-error:")

// MacroErrorKey
var MacroErrorKey = Intern("macro-error:")

// IOErrorKey
var IOErrorKey = Intern("io-error:")

// HttpErrorKey
var HTTPErrorKey = Intern("http-error:")

// InterruptKey
var InterruptKey = Intern("interrupt:")
