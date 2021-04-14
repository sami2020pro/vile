package vile

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

const ( /* Vile operation code or opcode */
	opcodeLiteral = iota // 0
	opcodeLocal	     // 1
	opcodeJumpFalse      // 2
	opcodeJump           // 3
	opcodeTailCall       // 4
	opcodeCall           // 5
	opcodeReturn         // 6
	opcodeClosure        // 7
	opcodePop            // 8
	opcodeGlobal         // 9
	opcodeDefGlobal      // 10
	opcodeSetLocal       // 11
	opcodeImport         // 12
	opcodeDefMacro       // 13
	opcodeVector         // 14
	opcodeStruct         // 15
	opcodeUndefGlobal    // 16
	opcodeCount          // 17
) /* Vile have 18 opcodes */

var LiteralSymbol = Intern("literal")
var LocalSymbol = Intern("local")
var JumpfalseSymbol = Intern("jumpfalse")
var JumpSymbol = Intern("jump")
var TailcallSymbol = Intern("tailcall")
var CallSymbol = Intern("call")
var ReturnSymbol = Intern("return")
var ClosureSymbol = Intern("closure")
var PopSymbol = Intern("pop")
var GlobalSymbol = Intern("global")
var DefglobalSymbol = Intern("defglobal")
var SetlocalSymbol = Intern("setlocal")
var ImportSymbol = Intern("import")
var DefmacroSymbol = Intern("macro")
var VectorSymbol = Intern("vector")
var StructSymbol = Intern("struct")
var UndefineSymbol = Intern("undefine")
var FuncSymbol = Intern("func")

var opsyms = initOpsyms()

func initOpsyms() []*Object {
	syms := make([]*Object, opcodeCount)
	syms[opcodeLiteral] = LiteralSymbol
	syms[opcodeLocal] = LocalSymbol
	syms[opcodeJumpFalse] = JumpfalseSymbol
	syms[opcodeJump] = JumpSymbol
	syms[opcodeTailCall] = TailcallSymbol
	syms[opcodeCall] = CallSymbol
	syms[opcodeReturn] = ReturnSymbol
	syms[opcodeClosure] = ClosureSymbol
	syms[opcodePop] = PopSymbol
	syms[opcodeGlobal] = GlobalSymbol
	syms[opcodeDefGlobal] = DefglobalSymbol
	syms[opcodeSetLocal] = SetlocalSymbol
	syms[opcodeImport] = ImportSymbol
	syms[opcodeDefMacro] = DefmacroSymbol
	syms[opcodeVector] = VectorSymbol
	syms[opcodeStruct] = StructSymbol
	syms[opcodeUndefGlobal] = UndefineSymbol
	return syms
}

// Code - compiled Vile bytecode
type Code struct {
	name     string
	ops      []int
	argc     int
	defaults []*Object // defaults => []*Object
	keys     []*Object // keys     => []*Object
}

// MakeCode - create a new code object
func MakeCode(argc int, defaults []*Object, keys []*Object, name string) *Object {
	var ops []int
	code := &Code{
		name,
		ops,
		argc,
		defaults, // nil for normal procs, empty for rest, and non-empty for optional/keyword
		keys,
	}
	result := new(Object)
	result.Type = CodeType // CodeType is the type of compiled code
	result.code = code
	return result
}

func (code *Code) signature() string {
	if code.name != "" {
		val := GetGlobal(Intern("*declarations*"))
		if val != nil && IsStruct(val) {
			sig, _ := Get(val, Intern(code.name))
			if sig != Null {
				return sig.String()
			}
		}
	}
	tmp := ""
	for i := 0; i < code.argc; i++ {
		tmp += " <any>"
	}
	if code.defaults != nil {
		tmp += " <any>*"
	}
	if tmp != "" {
		tmp = "(" + tmp[1:] + ")"
	} else {
		tmp = "()"
	}
	return tmp
}

func (code *Code) decompile(pretty bool) string {
	var buf bytes.Buffer
	code.decompileInto(&buf, "", pretty)
	s := buf.String()
	return strings.Replace(s, "("+FuncSymbol.text+" (\"\" 0 [] [])", "(code", 1)
}

func (code *Code) decompileInto(buf *bytes.Buffer, indent string, pretty bool) {
	indentAmount := "   "
	offset := 0
	max := len(code.ops)
	prefix := " "
	buf.WriteString(indent + "(" + FuncSymbol.text + " (")
	buf.WriteString(fmt.Sprintf("%q ", code.name))
	buf.WriteString(strconv.Itoa(code.argc))
	if code.defaults != nil {
		buf.WriteString(" ")
		buf.WriteString(fmt.Sprintf("%v", code.defaults))
	} else {
		buf.WriteString(" []")
	}
	if code.keys != nil {
		buf.WriteString(" ")
		buf.WriteString(fmt.Sprintf("%v", code.keys))
	} else {
		buf.WriteString(" []")
	}
	buf.WriteString(")")
	if pretty {
		indent = indent + indentAmount
		prefix = "\n" + indent
	}
	for offset < max {
		op := code.ops[offset]
		s := prefix + "(" + opsyms[op].text
		switch op {
		case opcodePop, opcodeReturn:
			buf.WriteString(s + ")")
			offset++
		case opcodeLiteral, opcodeDefGlobal, opcodeImport, opcodeGlobal, opcodeUndefGlobal, opcodeDefMacro:
			buf.WriteString(s + " " + Write(constants[code.ops[offset+1]]) + ")")
			offset += 2
		case opcodeCall, opcodeTailCall, opcodeJumpFalse, opcodeJump, opcodeVector, opcodeStruct:
			buf.WriteString(s + " " + strconv.Itoa(code.ops[offset+1]) + ")")
			offset += 2
		case opcodeLocal, opcodeSetLocal:
			buf.WriteString(s + " " + strconv.Itoa(code.ops[offset+1]) + " " + strconv.Itoa(code.ops[offset+2]) + ")")
			offset += 3
		case opcodeClosure:
			buf.WriteString(s)
			if pretty {
				buf.WriteString("\n")
			} else {
				buf.WriteString(" ")
			}
			indent2 := ""
			if pretty {
				indent2 = indent + indentAmount
			}
			constants[code.ops[offset+1]].code.decompileInto(buf, indent2, pretty)
			buf.WriteString(")")
			offset += 2
		default:
			panic(fmt.Sprintf("Bad instruction: %d", code.ops[offset]))
		}
	}
	buf.WriteString(")")
}

func (code *Code) String() string {
	return code.decompile(true)
	//	return fmt.Sprintf("(function (%d %v %s) %v)", code.argc, code.defaults, code.keys, code.ops)
}

func (code *Code) loadOps(lst *Object) error {
	for lst != EmptyList {
		instr := Car(lst)
		op := Car(instr)
		switch op {
		case ClosureSymbol:
			lstFunc := Cadr(instr)
			if Car(lstFunc) != FuncSymbol {
				return Error(SyntaxErrorKey, instr)
			}
			lstFunc = Cdr(lstFunc)
			funcParams := Car(lstFunc)
			var argc int
			var name string
			var defaults []*Object
			var keys []*Object
			var err error
			if IsSymbol(funcParams) {
				// legacy form, just the argc
				argc, err = AsIntValue(funcParams)
				if err != nil {
					return err
				}
				if argc < 0 {
					argc = -argc - 1
					defaults = make([]*Object, 0)
				}
			} else if IsList(funcParams) && ListLength(funcParams) == 4 {
				tmp := funcParams
				a := Car(tmp)
				tmp = Cdr(tmp)
				name, err = AsStringValue(a)
				if err != nil {
					return Error(SyntaxErrorKey, funcParams)
				}
				a = Car(tmp)
				tmp = Cdr(tmp)
				argc, err = AsIntValue(a)
				if err != nil {
					return Error(SyntaxErrorKey, funcParams)
				}
				a = Car(tmp)
				tmp = Cdr(tmp)
				if IsVector(a) {
					defaults = a.elements
				}
				a = Car(tmp)
				if IsVector(a) {
					keys = a.elements
				}
			} else {
				return Error(SyntaxErrorKey, funcParams)
			}
			fun := MakeCode(argc, defaults, keys, name)
			fun.code.loadOps(Cdr(lstFunc))
			code.emitClosure(fun)
		case LiteralSymbol:
			code.emitLiteral(Cadr(instr))
		case LocalSymbol:
			i, err := AsIntValue(Cadr(instr))
			if err != nil {
				return err
			}
			j, err := AsIntValue(Caddr(instr))
			if err != nil {
				return err
			}
			code.emitLocal(i, j)
		case SetlocalSymbol:
			i, err := AsIntValue(Cadr(instr))
			if err != nil {
				return err
			}
			j, err := AsIntValue(Caddr(instr))
			if err != nil {
				return err
			}
			code.emitSetLocal(i, j)
		case GlobalSymbol:
			sym := Cadr(instr)
			if IsSymbol(sym) {
				code.emitGlobal(sym)
			} else {
				return Error(GlobalSymbol, " argument 1 not a symbol: ", sym)
			}
		case UndefineSymbol:
			code.emitUndefGlobal(Cadr(instr))
		case JumpSymbol:
			loc, err := AsIntValue(Cadr(instr))
			if err != nil {
				return err
			}
			code.emitJump(loc)
		case JumpfalseSymbol:
			loc, err := AsIntValue(Cadr(instr))
			if err != nil {
				return err
			}
			code.emitJumpFalse(loc)
		case CallSymbol:
			argc, err := AsIntValue(Cadr(instr))
			if err != nil {
				return err
			}
			code.emitCall(argc)
		case TailcallSymbol:
			argc, err := AsIntValue(Cadr(instr))
			if err != nil {
				return err
			}
			code.emitTailCall(argc)
		case ReturnSymbol:
			code.emitReturn()
		case PopSymbol:
			code.emitPop()
		case DefglobalSymbol:
			code.emitDefGlobal(Cadr(instr))
		case DefmacroSymbol:
			code.emitDefMacro(Cadr(instr))
		case ImportSymbol:
			code.emitImport(Cadr(instr))
		default:
			panic(fmt.Sprintf("Bad instruction: %v", op))
		}
		lst = Cdr(lst)
	}
	return nil
}

func (code *Code) emitLiteral(val *Object) {
	code.ops = append(code.ops, opcodeLiteral)
	code.ops = append(code.ops, putConstant(val))
}

func (code *Code) emitGlobal(sym *Object) {
	code.ops = append(code.ops, opcodeGlobal)
	code.ops = append(code.ops, putConstant(sym))
}
func (code *Code) emitCall(argc int) {
	code.ops = append(code.ops, opcodeCall)
	code.ops = append(code.ops, argc)
}
func (code *Code) emitReturn() {
	code.ops = append(code.ops, opcodeReturn)
}
func (code *Code) emitTailCall(argc int) {
	code.ops = append(code.ops, opcodeTailCall)
	code.ops = append(code.ops, argc)
}
func (code *Code) emitPop() {
	code.ops = append(code.ops, opcodePop)
}
func (code *Code) emitLocal(i int, j int) {
	code.ops = append(code.ops, opcodeLocal)
	code.ops = append(code.ops, i)
	code.ops = append(code.ops, j)
}
func (code *Code) emitSetLocal(i int, j int) {
	code.ops = append(code.ops, opcodeSetLocal)
	code.ops = append(code.ops, i)
	code.ops = append(code.ops, j)
}
func (code *Code) emitDefGlobal(sym *Object) {
	code.ops = append(code.ops, opcodeDefGlobal)
	code.ops = append(code.ops, putConstant(sym))
}
func (code *Code) emitUndefGlobal(sym *Object) {
	code.ops = append(code.ops, opcodeUndefGlobal)
	code.ops = append(code.ops, putConstant(sym))
}
func (code *Code) emitDefMacro(sym *Object) {
	code.ops = append(code.ops, opcodeDefMacro)
	code.ops = append(code.ops, putConstant(sym))
}
func (code *Code) emitClosure(newCode *Object) {
	code.ops = append(code.ops, opcodeClosure)
	code.ops = append(code.ops, putConstant(newCode))
}
func (code *Code) emitJumpFalse(offset int) int {
	code.ops = append(code.ops, opcodeJumpFalse)
	loc := len(code.ops)
	code.ops = append(code.ops, offset)
	return loc
}
func (code *Code) emitJump(offset int) int {
	code.ops = append(code.ops, opcodeJump)
	loc := len(code.ops)
	code.ops = append(code.ops, offset)
	return loc
}
func (code *Code) setJumpLocation(loc int) {
	code.ops[loc] = len(code.ops) - loc + 1
}
func (code *Code) emitVector(alen int) {
	code.ops = append(code.ops, opcodeVector)
	code.ops = append(code.ops, alen)
}

func (code *Code) emitStruct(slen int) {
	code.ops = append(code.ops, opcodeStruct)
	code.ops = append(code.ops, slen)
}

func (code *Code) emitImport(sym *Object) {
	code.ops = append(code.ops, opcodeImport)
	code.ops = append(code.ops, putConstant(sym))
}
