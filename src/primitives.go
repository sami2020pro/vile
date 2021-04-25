package vile

// the primitive functions for the language

import (
	"fmt"
	"math"
	"os"
)

func InitPrimitives() {
	/* Whene we define a primitive more than one we got *warning*, e.g
		*** Warning: redefining  +  with a primitive
		DefineFunction("+", vileAdd, NumberType, NumberType, NumberType) // +
		DefineFunction("+", vileAdd, NumberType, NumberType, AnyType)    // +
	*/

	DefineMacro("quasiquote", vileQuasiquote)

	DefineGlobal("apply", Apply)
	DefineGlobal("callcc", CallCC)
	DefineGlobal("spawn", Spawn)

	DefineFunction("eval", vileEval, NullType, StringType)
	DefineFunction("exit", vileExit, NullType, NumberType)

	DefineFunction("type", vileType, TypeType, AnyType)

	DefineFunction("+", vileAdd, NumberType, NumberType, NumberType) // +
	DefineFunction("-", vileSub, NumberType, NumberType, NumberType) // -
	DefineFunction("*", vileMul, NumberType, NumberType, NumberType) // *
	DefineFunction("/", vileDiv, NumberType, NumberType, NumberType) // /
	DefineFunction("=", vileNumEqual, BooleanType, NumberType, NumberType) // =
	DefineFunction("<=", vileNumLessEqual, BooleanType, NumberType, NumberType) // <=
	DefineFunction(">=", vileNumGreaterEqual, BooleanType, NumberType, NumberType) // >=
	DefineFunction(">", vileNumGreater, BooleanType, NumberType, NumberType) // >
	DefineFunction("<", vileNumLess, BooleanType, NumberType, NumberType) // <

	DefineFunction("&", vileBinaryAndOperator, NumberType, NumberType, NumberType)
	DefineFunction("|", vileBinaryOrOperator, NumberType, NumberType, NumberType)
	DefineFunction("^", vileBinaryXorOperator, NumberType, NumberType, NumberType)
	DefineFunction("<<", vileBinaryLeftShiftOperator, NumberType, NumberType, NumberType)
	DefineFunction(">>", vileBinaryRightShiftOperator, NumberType, NumberType, NumberType)

	DefineFunction("len", vileLen, NumberType, StringType)

	DefineFunction("cons", vileCons, ListType, AnyType, ListType)
	DefineFunction("car", vileCar, AnyType, ListType)
	DefineFunction("cdr", vileCdr, ListType, ListType)

	DefineFunction("round", vileRound, NumberType, NumberType)
	DefineFunction("ceil", vileCeil, NumberType, NumberType)
	DefineFunction("floor", vileFloor, NumberType, NumberType)

	DefineFunction("log", vileLog, NumberType, NumberType)
	DefineFunction("sin", vileSin, NumberType, NumberType)
	DefineFunction("cos", vileCos, NumberType, NumberType)

	DefineFunction("inc", vileInc, NumberType, NumberType)
	DefineFunction("dec", vileDec, NumberType, NumberType)

	DefineFunction("compile", vileCompile, CodeType, AnyType)

	DefineFunctionRestArgs("puts", vilePuts, NullType, AnyType)
	DefineFunctionRestArgs("put", vilePut, NullType, AnyType)
	DefineFunctionRestArgs("list", vileList, ListType, AnyType)
	DefineFunctionRestArgs("concat", vileConcat, ListType, ListType)

	DefineFunction("load", vileLoad, StringType, AnyType)

	/* TESTS */
	DefineFunctionRestArgs("struct", vileStruct, StructType, AnyType)
	DefineFunction("make_struct", vileMakeStruct, StructType, NumberType)

	DefineFunction("char?", vileCharP, BooleanType, AnyType)
	DefineFunction("to_char", vileToChar, CharacterType, AnyType)

	DefineFunction("reverse_list", vileReverseList, ListType, ListType)
	DefineFunction("reverse_string", vileReverseString, StringType, StringType)
}

func vileQuasiquote(argv []*Object) (*Object, error) {
	return expandQuasiquote(argv[0])
}

func vileEval(argv []*Object) (*Object, error) {
	err := RunStringEval(argv[0].text)
	if err != nil {
		panic(err)
	}

	return argv[0], nil
}

func vileExit(argv []*Object) (*Object, error) {
	if argv[0].fval == 1 {
		os.Exit(0)
	}

	return Null, nil
}

func vileType(argv []*Object) (*Object, error) {
	return argv[0].Type, nil
}

func vileAdd(argv []*Object) (*Object, error) {
	return Number(argv[0].fval + argv[1].fval), nil
}


func vileBinaryAndOperator(argv []*Object) (*Object, error) {
	x := int(argv[0].fval) & int(argv[1].fval)
	return Number(float64(x)), nil
}

func vileBinaryOrOperator(argv []*Object) (*Object, error) {
	x := int(argv[0].fval) | int(argv[1].fval)
	return Number(float64(x)), nil
}

func vileBinaryXorOperator(argv []*Object) (*Object, error) {
	x := int(argv[0].fval) ^ int(argv[1].fval)
	return Number(float64(x)), nil
}

func vileBinaryLeftShiftOperator(argv []*Object) (*Object, error) {
	x := int(argv[0].fval) << int(argv[1].fval)
	return Number(float64(x)), nil
}

func vileBinaryRightShiftOperator(argv []*Object) (*Object, error) {
	x := int(argv[0].fval) >> int(argv[1].fval)
	return Number(float64(x)), nil
}

func vileSub(argv []*Object) (*Object, error) {
	return Number(argv[0].fval - argv[1].fval), nil
}

func vileMul(argv []*Object) (*Object, error) {
	return Number(argv[0].fval * argv[1].fval), nil
}

func vileDiv(argv []*Object) (*Object, error) {
	return Number(argv[0].fval / argv[1].fval), nil
}

func vileNumEqual(argv []*Object) (*Object, error) {
	if NumberEqual(argv[0].fval, argv[1].fval) {
		return True, nil
	}

	return False, nil
}

func vileNumLess(argv []*Object) (*Object, error) {
	if argv[0].fval < argv[1].fval {
		return True, nil
	}

	return False, nil
}

func vileNumLessEqual(argv []*Object) (*Object, error) {
	if argv[0].fval <= argv[1].fval {
		return True, nil
	}

	return False, nil
}

func vileNumGreater(argv []*Object) (*Object, error) {
	if argv[0].fval > argv[1].fval {
		return True, nil
	}

	return False, nil
}

func vileNumGreaterEqual(argv []*Object) (*Object, error) {
	if argv[0].fval >= argv[1].fval {
		return True, nil
	}

	return False, nil
}

func vileLen(argv []*Object) (*Object, error) {
	return Number(float64(len(argv[0].text))), nil
}

func vileCons(argv []*Object) (*Object, error) {
	return Cons(argv[0], argv[1]), nil
}

func vileCar(argv []*Object) (*Object, error) {
	lst := argv[0]
	if lst == EmptyList {
		return Null, nil
	}

	return lst.car, nil
}

func vileCdr(argv []*Object) (*Object, error) {
	lst := argv[0]
	if lst == EmptyList {
		return lst, nil
	}

	return lst.cdr, nil
}

func vileRound(argv []*Object) (*Object, error) {
	return Number(math.Round(argv[0].fval)), nil
}

func vileCeil(argv []*Object) (*Object, error) {
	return Number(math.Ceil(argv[0].fval)), nil
}

func vileFloor(argv []*Object) (*Object, error) {
	return Number(math.Floor(argv[0].fval)), nil
}

func vileLog(argv []*Object) (*Object, error) {
	return Number(math.Log(argv[0].fval)), nil
}

func vileSin(argv []*Object) (*Object, error) {
	return Number(math.Sin(argv[0].fval)), nil
}

func vileCos(argv []*Object) (*Object, error) {
	return Number(math.Cos(argv[0].fval)), nil
}

func vileInc(argv []*Object) (*Object, error) {
	return Number(argv[0].fval + 1), nil
}

func vileDec(argv []*Object) (*Object, error) {
	return Number(argv[0].fval - 1), nil
}

/* FIXED: check the type and convert it to String and print it */
func vilePuts(argv []*Object) (*Object, error) {
	var stringVar string

	for _, o := range argv {
		stringVar = fmt.Sprintf("%v", o)
		fmt.Print(stringVar)
	} fmt.Println()

	return Null, nil
}

/* FIXED: check the type and convert it to String and print it */
func vilePut(argv []*Object) (*Object, error) {
    var stringVar string

	for _, o := range argv {
		stringVar = fmt.Sprintf("%v", o)
        fmt.Print(stringVar)
    }

    return Null, nil
}

func vileList(argv []*Object) (*Object, error) {
	argc := len(argv)
	p := EmptyList
	for i := argc - 1; i >= 0; i-- {
		p = Cons(argv[i], p)
	}

	return p, nil
}

func vileConcat(argv []*Object) (*Object, error) {
	result := EmptyList
	tail := result
	for _, lst := range argv {
		for lst != EmptyList {
			if tail == EmptyList {
				result = List(lst.car)
				tail = result
			} else {
				tail.cdr = List(lst.car)
				tail = tail.cdr
			}
			lst = lst.cdr
		}
	}

	return result, nil
}

func vileCompile(argv []*Object) (*Object, error) {
	expanded, err := Macroexpand(argv[0])
	if err != nil {
		return nil, err
	}

	return Compile(expanded)
}

func vileLoad(argv []*Object) (*Object, error) {
	err := Load(argv[0].text)
	return argv[0], err
}

/* TESTS */
func vileStruct(argv []*Object) (*Object, error) {
	return Struct(argv)
}

func vileMakeStruct(argv []*Object) (*Object, error) {
	return MakeStruct(int(argv[0].fval)), nil
}

func vileCharP(argv []*Object) (*Object, error) {
	if IsCharacter(argv[0]) {
		return True, nil
	}
	return False, nil
}

func vileToChar(argv []*Object) (*Object, error) {
	return ToCharacter(argv[0])
}

func vileReverseList(argv []*Object) (*Object, error) {
	return ReverseList(argv[0]), nil
}

func vileReverseString(argv []*Object) (*Object, error) {
	return ReverseString(argv[0].text), nil
}
