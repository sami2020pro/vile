package vile

import (
	"bytes"
	"fmt"
	"os"
	"time"
)

/*
 * vm => virtual machine
 * sp => stack pointer
 * ip => instruction pointer
 * pc => program counter
 */

/*
 * TailCall => Wikipedia
 */

var trace bool
var optimize bool

var interrupted = false
var interrupts chan os.Signal

func checkInterrupt() bool {
	if interrupts != nil {
		select {
		case msg := <-interrupts:
			if msg != nil {
				interrupted = true
				return true
			}
		default:
			return false
		}
	}
	return false
}

func str(o interface{}) string {
	if lob, ok := o.(*Object); ok {
		return lob.String()
	}
	return fmt.Sprintf("%v", o)
}

func Print(args ...interface{}) {
	max := len(args) - 1
	for i := 0; i < max; i++ {
		fmt.Print(str(args[i]))
	}
	fmt.Print(str(args[max]))
}

func Println(args ...interface{}) {
	max := len(args) - 1
	for i := 0; i < max; i++ {
		fmt.Print(str(args[i]))
	}
	fmt.Println(str(args[max]))
}

func Fatal(args ...interface{}) {
	Println(args...)
	Cleanup()
	exit(1)
}

// Continuation -
type continuation struct {
	ops   []int
	stack []*Object
	pc    int
}

func Closure(code *Code, frame *frame) *Object {
	fun := new(Object)
	fun.Type = FunctionType
	fun.code = code
	fun.frame = frame
	return fun
}

func Continuation(frame *frame, ops []int, pc int, stack []*Object) *Object {
	fun := new(Object)
	fun.Type = FunctionType
	cont := new(continuation)
	cont.ops = ops
	cont.stack = make([]*Object, len(stack))
	copy(cont.stack, stack)
	cont.pc = pc
	fun.frame = frame
	fun.continuation = cont
	return fun
}

const defaultStackSize = 1000

// VM - the Vile VM
type vm struct {
	stackSize int
}

func VM(stackSize int) *vm {
	return &vm{stackSize}
}

// Apply is a primitive instruction to apply a function to a list of arguments
var Apply = &Object{Type: FunctionType}

// CallCC is a primitive instruction to executable (restore) a continuation
var CallCC = &Object{Type: FunctionType}

// Apply is a primitive instruction to apply a function to a list of arguments
var Spawn = &Object{Type: FunctionType}

func functionSignature(f *Object) string {
	if f.primitive != nil {
		return f.primitive.signature
	}
	if f.code != nil {
		return f.code.signature()
	}
	if f.continuation != nil {
		return "(<function>) <any>"
	}
	if f == Apply {
		return "(<any>*) <list>"
	}
	if f == CallCC {
		return "(<function>) <any>"
	}
	if f == Spawn {
		return "(<function> <any>*) <null>"
	}
	panic("Bad function")
}

func functionToString(f *Object) string {
	if f.primitive != nil {
		return "#[function " + f.primitive.name + "]"
	}
	if f.code != nil {
		n := f.code.name
		if n == "" {
			return fmt.Sprintf("#[function]")
		}
		return fmt.Sprintf("#[function %s]", n)
	}
	if f.continuation != nil {
		return "#[continuation]"
	}
	if f == Apply {
		return "#[function apply]"
	}
	if f == CallCC {
		return "#[function callcc]"
	}
	if f == Spawn {
		return "#[function spawn]"
	}
	panic("Bad function")
}

// PrimitiveFunction is the native go function signature for all Vile primitive functions
type PrimitiveFunction func(argv []*Object) (*Object, error)

// Primitive - a primitive function, written in Go, callable by VM
type primitive struct { // <function>
	name      string
	fun       PrimitiveFunction
	signature string
	idx       int
	argc      int       // -1 means the primitive itself checks the args (legacy mode)
	result    *Object   // if set the type of the result
	args      []*Object // if set, the length must be for total args (both required and optional). The type (or <any>) for each
	rest      *Object   // if set, then any number of this type can follow the normal args. Mutually incompatible with defaults/keys
	defaults  []*Object // if set, then that many optional args beyond argc have these default values
	keys      []*Object // if set, then it must match the size of defaults, and these are the keys
}

func functionSignatureFromTypes(result *Object, args []*Object, rest *Object) string {
	sig := "("
	for i, t := range args {
		if !IsType(t) {
			panic("not a type: " + t.String())
		}
		if i > 0 {
			sig += " "
		}
		sig += t.text
	}
	if rest != nil {
		if !IsType(rest) {
			panic("not a type: " + rest.String())
		}
		if sig != "(" {
			sig += " "
		}
		sig += rest.text + "*"
	}
	sig += ") "
	if !IsType(result) {
		panic("not a type: " + result.String())
	}
	sig += result.text
	return sig
}

/* used in module.go - five places */
func Primitive(name string, fun PrimitiveFunction, result *Object, args []*Object, rest *Object, defaults []*Object, keys []*Object) *Object {
	// the rest type indicates arguments past the end of args will all have the given type. the length must be checked by primitive
	// -> they are all optional, then. So, (<any>+) must be expressed as (<any> <any>*)
	idx := len(primitives)
	argc := len(args)
	if defaults != nil {
		defc := len(defaults)
		if defc > argc {
			panic("more default argument values than types: " + name)
		}
		if keys != nil {
			if len(keys) != defc {
				panic("Argument keys must have same length as argument defaults")
			}
		}
		argc = argc - defc
		for i := 0; i < defc; i++ {
			t := args[argc+i]
			if t != AnyType && defaults[i].Type != t {
				panic("argument default's type (" + defaults[i].Type.text + ") doesn't match declared type (" + t.text + ")")
			}
		}
	} else {
		if keys != nil {
			panic("Cannot have argument keys without argument defaults")
		}
	}
	signature := functionSignatureFromTypes(result, args, rest) // functionSignatureFromTypes was defined in runtime.go - 184 line
	prim := &primitive{name, fun, signature, idx, argc, result, args, rest, defaults, keys}
	primitives = append(primitives, prim)
	return &Object{Type: FunctionType, primitive: prim}
}

/* Ruby YARV:
 * Virtual machines simulate the way physical machines execute method calls,
 * each executing method (or block) pushes a stack frame into the stack. 
 */

type frame struct { /* If you want to know more about this structure read about "Call stack" in Wikipedia */
	locals    *frame
	previous  *frame
	code      *Code
	ops       []int
	elements  []*Object
	firstfive [5]*Object
	pc        int
}

func (frame *frame) String() string {
	var buf bytes.Buffer
	buf.WriteString("#[frame ")
	if frame.code != nil {
		if frame.code.name != "" {
			buf.WriteString(" " + frame.code.name)
		} else {
			buf.WriteString(" (anonymous code)")
		}
	} else {
		buf.WriteString(" (no code)")
	}
	buf.WriteString(fmt.Sprintf(" previous: %v", frame.previous))
	buf.WriteString("]")
	return buf.String()
}

func buildFrame(env *frame, pc int, ops []int, fun *Object, argc int, stack []*Object, sp int) (*frame, error) {
	f := new(frame)
	f.previous = env
	f.pc = pc
	f.ops = ops
	f.locals = fun.frame
	f.code = fun.code
	expectedArgc := fun.code.argc
	defaults := fun.code.defaults
	// println("buildFrame check point: ", defaults)
	if defaults == nil {
		if argc != expectedArgc {
			return nil, Error(ArgumentErrorKey, "Wrong number of args to ", fun, " (expected ", expectedArgc, ", got ", argc, ")")
		}
		if argc <= 5 {
			f.elements = f.firstfive[:]
		} else {
			f.elements = make([]*Object, argc)
		}
		copy(f.elements, stack[sp:sp+argc])
		return f, nil
	}
	keys := fun.code.keys
	rest := false
	extra := len(defaults)
	if extra == 0 {
		rest = true
		extra = 1
	}
	if argc < expectedArgc {
		if extra > 0 {
			return nil, Error(ArgumentErrorKey, "Wrong number of args to ", fun, " (expected at least ", expectedArgc, ", got ", argc, ")")
		}
		return nil, Error(ArgumentErrorKey, "Wrong number of args to ", fun, " (expected ", expectedArgc, ", got ", argc, ")")
	}
	totalArgc := expectedArgc + extra
	el := make([]*Object, totalArgc)
	end := sp + expectedArgc
	if rest {
		copy(el, stack[sp:end])
		restElements := stack[end : sp+argc]
		el[expectedArgc] = ListFromValues(restElements)
	} else if keys != nil {
		bindings := stack[sp+expectedArgc : sp+argc]
		if len(bindings)%2 != 0 {
			return nil, Error(ArgumentErrorKey, "Bad keyword argument(s): ", bindings)
		}
		copy(el, stack[sp:sp+expectedArgc]) // the required ones
		for i := expectedArgc; i < totalArgc; i++ {
			el[i] = defaults[i-expectedArgc]
		}
		for i := expectedArgc; i < argc; i += 2 {
			key, err := ToSymbol(stack[sp+i])
			if err != nil {
				return nil, Error(ArgumentErrorKey, "Bad keyword argument: ", stack[sp+1])
			}
			gotit := false
			for j := 0; j < extra; j++ {
				if keys[j] == key {
					el[expectedArgc+j] = stack[sp+i+1]
					gotit = true
					break
				}
			}
			if !gotit {
				return nil, Error(ArgumentErrorKey, "Undefined keyword argument: ", key)
			}
		}
	} else {
		copy(el, stack[sp:sp+argc])
		for i := argc; i < totalArgc; i++ {
			el[i] = defaults[i-expectedArgc]
		}
	}
	f.elements = el
	return f, nil
}

func addContext(env *frame, err error) error {
	if e, ok := err.(*Object); ok {
		if env.code != nil {
			if env.code.name != "throw" {
				e.text = env.code.name
			} else if env.previous != nil {
				if env.previous.code != nil {
					e.text = env.previous.code.name
				}
			}
		}
	}
	return err
}

func (vm *vm) keywordCall(fun *Object, argc int, pc int, stack []*Object, sp int) (int, int, error) {
	if argc != 1 {
		return 0, 0, Error(ArgumentErrorKey, fun.text, " expected 1 argument, got ", argc)
	}
	v, err := Get(stack[sp], fun)
	if err != nil {
		return 0, 0, err
	}
	stack[sp] = v
	return pc, sp, nil
}

func argcError(name string, min int, max int, provided int) error {
	s := "1 argument"
	if min == max {
		if min != 1 {
			s = fmt.Sprintf("%d arguments", min)
		}
	} else if max < 0 {
		s = fmt.Sprintf("%d or more arguments", min)
	} else {
		s = fmt.Sprintf("%d to %d arguments", min, max)
	}
	return Error(ArgumentErrorKey, fmt.Sprintf("%s expected %s, got %d", name, s, provided))
}

func (vm *vm) callPrimitive(prim *primitive, argv []*Object) (*Object, error) {
	// println(prim.defaults)
	if prim.defaults != nil {
		return vm.callPrimitiveWithDefaults(prim, argv)
	}
	argc := len(argv)
	if argc != prim.argc {
		return nil, argcError(prim.name, prim.argc, prim.argc, argc)
	}
	for i, arg := range argv {
		t := prim.args[i]
		if t != AnyType && arg.Type != t {
			return nil, Error(ArgumentErrorKey, fmt.Sprintf("%s expected a %s for argument %d, got a %s", prim.name, prim.args[i].text, i+1, argv[i].Type.text))
		}
	}
	return prim.fun(argv)
}

func (vm *vm) callPrimitiveWithDefaults(prim *primitive, argv []*Object) (*Object, error) {
	provided := len(argv)
	minargc := prim.argc
	if len(prim.defaults) == 0 {
		rest := prim.rest
		if provided < minargc {
			return nil, argcError(prim.name, minargc, -1, provided)
		}
		for i := 0; i < minargc; i++ {
			t := prim.args[i]
			arg := argv[i]
			if t != AnyType && arg.Type != t {
				return nil, Error(ArgumentErrorKey, fmt.Sprintf("%s expected a %s for argument %d, got a %s", prim.name, prim.args[i].text, i+1, argv[i].Type.text))
			}
		}
		if rest != AnyType {
			for i := minargc; i < provided; i++ {
				arg := argv[i]
				if arg.Type != rest {
					return nil, Error(ArgumentErrorKey, fmt.Sprintf("%s expected a %s for argument %d, got a %s", prim.name, rest.text, i+1, argv[i].Type.text))
				}
			}
		}
		return prim.fun(argv)
	}
	maxargc := len(prim.args)
	if provided < minargc {
		return nil, argcError(prim.name, minargc, maxargc, provided)
	}
	newargs := make([]*Object, maxargc)
	if prim.keys != nil {
		j := 0
		copy(newargs, argv[:minargc])
		for i := minargc; i < maxargc; i++ {
			newargs[i] = prim.defaults[j]
			j++
		}
		j = minargc // the first key arg
		ndefaults := len(prim.defaults)
		for j < provided {
			k := argv[j]
			j++
			if j == provided {
				return nil, Error(ArgumentErrorKey, "mismatched keyword/value pair in argument list")
			}
			if k.Type != KeywordType {
				return nil, Error(ArgumentErrorKey, "expected keyword, got a "+k.Type.text)
			}
			gotit := false
			for i := 0; i < ndefaults; i++ {
				if prim.keys[i] == k {
					gotit = true
					newargs[i+minargc] = argv[j]
					j++
					break
				}
			}
			if !gotit {
				return nil, Error(ArgumentErrorKey, prim.name, " accepts ", prim.keys, " as keyword arg(s), not ", k)
			}
		}
		argv = newargs
	} else {
		if provided > maxargc {
			return nil, argcError(prim.name, minargc, maxargc, provided)
		}
		copy(newargs, argv)
		j := 0
		for i := provided; i < maxargc; i++ {
			newargs[i] = prim.defaults[j]
			j++
		}
		argv = newargs
	}
	for i, arg := range argv {
		t := prim.args[i]
		if t != AnyType && arg.Type != t {
			return nil, Error(ArgumentErrorKey, fmt.Sprintf("%s expected a %s for argument %d, got a %s", prim.name, prim.args[i].text, i+1, argv[i].Type.text))
		}
	}
	return prim.fun(argv)
}

func (vm *vm) funcall(fun *Object, argc int, ops []int, savedPc int, stack []*Object, sp int, env *frame) ([]int, int, int, *frame, error) {
opcodeCallAgain: // If you don't know about this line or code read about Label in Go
	if fun.Type == FunctionType {
		if fun.code != nil {
			if interrupted || checkInterrupt() {
				return nil, 0, 0, nil, addContext(env, Error(InterruptKey)) // not catchable
			}
			if fun.code.defaults == nil { // IMPORTANT - read about subroutine in Wikipedia
				f := new(frame)
				f.previous = env
				f.pc = savedPc // savedPc = `saved program counter`
				f.ops = ops
				f.locals = fun.frame
				f.code = fun.code
				expectedArgc := fun.code.argc
				if argc != expectedArgc {
					return nil, 0, 0, nil, Error(ArgumentErrorKey, "Wrong number of args to ", fun, " (expected ", expectedArgc, ", got ", argc, ")")
				}
				if argc <= 5 {
					f.elements = f.firstfive[:argc]
				} else {
					f.elements = make([]*Object, argc)
				}
				endSp := sp + argc
				copy(f.elements, stack[sp:endSp])
				return fun.code.ops, 0, endSp, f, nil
			}
			f, err := buildFrame(env, savedPc, ops, fun, argc, stack, sp)
			if err != nil {
				return vm.catch(err, stack, env)
			}
			sp += argc
			env = f
			ops = fun.code.ops
			return ops, 0, sp, env, err
		}
		if fun.primitive != nil {
			val, err := vm.callPrimitive(fun.primitive, stack[sp:sp+argc])
			if err != nil {
				return vm.catch(err, stack, env)
			}
			sp = sp + argc - 1
			stack[sp] = val
			return ops, savedPc, sp, env, err
		}
		if fun == Apply {
			if argc < 2 {
				err := Error(ArgumentErrorKey, "apply expected at least 2 arguments, got ", argc)
				return vm.catch(err, stack, env)
			}
			fun = stack[sp]
			args := stack[sp+argc-1]
			if !IsList(args) {
				err := Error(ArgumentErrorKey, "apply expected a <list> as its final argument")
				return vm.catch(err, stack, env)
			}
			arglist := args
			for i := argc - 2; i > 0; i-- {
				arglist = Cons(stack[sp+i], arglist)
			}
			sp += argc
			argc = ListLength(arglist)
			i := 0
			sp -= argc
			for arglist != EmptyList {
				stack[sp+i] = arglist.car
				i++
				arglist = arglist.cdr
			}
			goto opcodeCallAgain
		}
		if fun == CallCC {
			if argc != 1 {
				err := Error(ArgumentErrorKey, "callcc expected 1 argument, got ", argc)
				return vm.catch(err, stack, env)
			}
			fun = stack[sp]
			stack[sp] = Continuation(env, ops, savedPc, stack[sp+1:])
			goto opcodeCallAgain
		}
		if fun.continuation != nil {
			if argc != 1 {
				err := Error(ArgumentErrorKey, "#[continuation] expected 1 argument, got ", argc)
				return vm.catch(err, stack, env)
			}
			arg := stack[sp]
			sp = len(stack) - len(fun.continuation.stack)
			segment := stack[sp:]
			copy(segment, fun.continuation.stack)
			sp--
			stack[sp] = arg
			return fun.continuation.ops, fun.continuation.pc, sp, fun.frame, nil
		}
		if fun == Spawn {
			err := vm.spawn(stack[sp], argc-1, stack, sp+1)
			if err != nil {
				return vm.catch(err, stack, env)
			}
			sp = sp + argc - 1
			stack[sp] = Null
			return ops, savedPc, sp, env, err
		}
		panic("unsupported instruction")
	}
	if fun.Type == KeywordType {
		if argc != 1 {
			err := Error(ArgumentErrorKey, fun.text, " expected 1 argument, got ", argc)
			return vm.catch(err, stack, env)
		}
		v, err := Get(stack[sp], fun)
		if err != nil {
			return vm.catch(err, stack, env)
		}
		stack[sp] = v
		return ops, savedPc, sp, env, err
	}
	err := Error(ArgumentErrorKey, "Not a function: ", fun)
	return vm.catch(err, stack, env)
}

func (vm *vm) tailcall(fun *Object, argc int, ops []int, stack []*Object, sp int, env *frame) ([]int, int, int, *frame, error) {
opcodeTailCallAgain: // opcodeTailCallAgain label
	if fun.Type == FunctionType {
		if fun.code != nil {
			if fun.code.defaults == nil && fun.code == env.code { // self-tail-call - we can reuse the frame.
				expectedArgc := fun.code.argc
				if argc != expectedArgc {
					return nil, 0, 0, nil, Error(ArgumentErrorKey, "Wrong number of args to ", fun, " (expected ", expectedArgc, ", got ", argc, ")")
				}
				endSp := sp + argc
				copy(env.elements, stack[sp:endSp])
				return fun.code.ops, 0, endSp, env, nil
			}
			f, err := buildFrame(env.previous, env.pc, env.ops, fun, argc, stack, sp) // make a frame
			if err != nil {
				return vm.catch(err, stack, env)
			}
			sp += argc
			return fun.code.ops, 0, sp, f, nil
		}
		if fun.primitive != nil {
			val, err := vm.callPrimitive(fun.primitive, stack[sp:sp+argc])
			if err != nil {
				return vm.catch(err, stack, env)
			}
			sp = sp + argc - 1
			stack[sp] = val
			return env.ops, env.pc, sp, env.previous, nil
		}
		if fun == Apply {
			if argc < 2 {
				err := Error(ArgumentErrorKey, "apply expected at least 2 arguments, got ", argc)
				return vm.catch(err, stack, env)
			}
			fun = stack[sp]
			args := stack[sp+argc-1]
			if !IsList(args) {
				err := Error(ArgumentErrorKey, "apply expected its last argument to be a <list>")
				return vm.catch(err, stack, env)
			}
			arglist := args
			for i := argc - 2; i > 0; i-- {
				arglist = Cons(stack[sp+i], arglist)
			}
			sp += argc
			argc = ListLength(arglist)
			i := 0
			sp -= argc
			for arglist != EmptyList {
				stack[sp+i] = arglist.car
				i++
				arglist = arglist.cdr
			}
			goto opcodeTailCallAgain
		}
		if fun.continuation != nil {
			if argc != 1 {
				err := Error(ArgumentErrorKey, "#[continuation] expected 1 argument, got ", argc)
				return vm.catch(err, stack, env)
			}
			arg := stack[sp]
			sp = len(stack) - len(fun.continuation.stack)
			segment := stack[sp:]
			copy(segment, fun.continuation.stack)
			sp--
			stack[sp] = arg
			return fun.continuation.ops, fun.continuation.pc, sp, fun.frame, nil
		}
		if fun == CallCC {
			if argc != 1 {
				err := Error(ArgumentErrorKey, "callcc expected 1 argument, got ", argc)
				return vm.catch(err, stack, env)
			}
			fun = stack[sp]
			stack[sp] = Continuation(env.previous, env.ops, env.pc, stack[sp:])
			goto opcodeTailCallAgain
		}
		if fun == Spawn {
			err := vm.spawn(stack[sp], argc-1, stack, sp+1)
			if err != nil {
				return vm.catch(err, stack, env)
			}
			sp = sp + argc - 1
			stack[sp] = Null
			return env.ops, env.pc, sp, env.previous, nil
		}
		panic("Bad function")
	}
	if fun.Type == KeywordType {
		if argc != 1 {
			err := Error(ArgumentErrorKey, fun.text, " expected 1 argument, got ", argc)
			return vm.catch(err, stack, env)
		}
		v, err := Get(stack[sp], fun)
		if err != nil {
			return vm.catch(err, stack, env)
		}
		stack[sp] = v
		return env.ops, env.pc, sp, env.previous, nil
	}
	err := Error(ArgumentErrorKey, "Not a function:", fun)
	return vm.catch(err, stack, env)
}

func (vm *vm) keywordTailcall(fun *Object, argc int, ops []int, stack []*Object, sp int, env *frame) ([]int, int, int, *frame, error) {
	if argc != 1 {
		err := Error(ArgumentErrorKey, fun.text, " expected 1 argument, got ", argc)
		return vm.catch(err, stack, env)
	}
	v, err := Get(stack[sp], fun)
	if err != nil {
		return vm.catch(err, stack, env)
	}
	stack[sp] = v
	return env.ops, env.pc, sp, env.previous, nil
}

func execCompileTime(code *Code, arg *Object) (*Object, error) {
	args := []*Object{arg}
	prev := verbose
	verbose = false
	res, err := exec(code, args)
	verbose = prev
	return res, err
}

func (vm *vm) catch(err error, stack []*Object, env *frame) ([]int, int, int, *frame, error) {
	errobj, ok := err.(*Object)
	if !ok {
		errobj = MakeError(ErrorKey, String(err.Error()))
	}
	handler := GetGlobal(Intern("*top-handler*"))
	if handler != nil && handler.Type == FunctionType {
		if handler.code != nil {
			if handler.code.argc == 1 {
				sp := len(stack) - 1
				stack[sp] = errobj
				return vm.funcall(handler, 1, nil, 0, stack, sp, nil)
			}
		}
	}
	return nil, 0, 0, nil, addContext(env, err)
}

func (vm *vm) spawn(fun *Object, argc int, stack []*Object, sp int) error {
	if fun.Type == FunctionType {
		if fun.code != nil {
			env, err := buildFrame(nil, 0, nil, fun, argc, stack, sp)
			if err != nil {
				return err
			}
			go func(code *Code, env *frame) {
				vm := VM(defaultStackSize)
				_, err := vm.exec(code, env)
				if err != nil {
//					println("; [*** error in spawned function '", code.name, "': ", err, "]")
				} else if verbose {
//					println("; [spawned function '", code.name, "' exited cleanly]")
				}
			}(fun.code, env)
			return nil
		}
		// spawning callcc, apply, and spawn instructions not supported.
		//? spawning primitives not supported. Is that important?
	}
	return Error(ArgumentErrorKey, "Bad function for spawn: ", fun)
}

func exec(code *Code, args []*Object) (*Object, error) {
	vm := VM(defaultStackSize) // virtual machine
	if len(args) != code.argc {
		return nil, Error(ArgumentErrorKey, "Wrong number of arguments")
	}
	env := new(frame)
	env.elements = make([]*Object, len(args))
	copy(env.elements, args)
	env.code = code
	startTime := time.Now()
	result, err := vm.exec(code, env) // exec method - runtime.go
	dur := time.Since(startTime)
	if err != nil {
		return nil, err
	}
	if result == nil {
		panic("result should never be nil if no error")
	}
	if verbose { // verbose mode in Vile dialect
		println("; executed in ", dur)
		if !interactive {
			println("; => ", result)
		}
	}
	return result, err
}

func (vm *vm) exec(code *Code, env *frame) (*Object, error) {
	if !optimize || verbose || trace { // check optimize and verbose and trace booleans
		return vm.instrumentedExec(code, env)
	}
	stack := make([]*Object, vm.stackSize) // stack
	sp := vm.stackSize // the sp is the `stack pointer`
	ops := code.ops // `operation codes`
	pc := 0 // `program `counter`
	var err error
	for { // until exec all ops
		op := ops[pc]
		if op == opcodeCall { // CALL
//			println("opcodeCall")
			argc := ops[pc+1]
			fun := stack[sp]
//			fmt.Printf("\n\n opcodeCall stack runtime.go checkpoint: %v and %T \n\n", stack[sp], stack[sp])
			if fun.primitive != nil { // primitives
//			fmt.Printf("\n\n opcodeCall stack runtime.go checkpoint: %v and %T \n\n", stack[sp], stack[sp])
				nextSp := sp + argc
				val, err := vm.callPrimitive(fun.primitive, stack[sp+1:nextSp+1])
				if err != nil {
					ops, pc, sp, env, err = vm.catch(err, stack, env)
					if err != nil {
						return nil, err
					}
				}
				stack[nextSp] = val
				sp = nextSp
				pc += 2
			} else if fun.Type == FunctionType { // defined in data.go
				ops, pc, sp, env, err = vm.funcall(fun, argc, ops, pc+2, stack, sp+1, env) // call function
				if err != nil {
					return nil, err
				}
			} else if fun.Type == KeywordType { // defined in data.go
				pc, sp, err = vm.keywordCall(fun, argc, pc+2, stack, sp+1) // call keyword
				if err != nil {
					ops, pc, sp, env, err = vm.catch(err, stack, env)
					if err != nil {
						return nil, err
					}
				}
			} else { // e.g: (1)
				ops, pc, sp, env, err = vm.catch(Error(ArgumentErrorKey, "Not callable: ", fun), stack, env) // catch
				if err != nil { // log err
					return nil, err
				}
			}
		} else if op == opcodeGlobal { // GObjectAL
			sym := constants[ops[pc+1]]
			sp--
			stack[sp] = sym.car
			pc += 2
		} else if op == opcodeLocal {
//			println("opcodelocal")
			tmpEnv := env
			i := ops[pc+1]
			for i > 0 {
				tmpEnv = tmpEnv.locals
				i--
			}
			j := ops[pc+2]
			val := tmpEnv.elements[j]
			sp--
			stack[sp] = val
			pc += 3
		} else if op == opcodeJumpFalse {
			b := stack[sp]
			sp++
			if b == False {
				pc += ops[pc+1]
			} else {
				pc += 2
			}
		} else if op == opcodePop { // Pop
//			println("opcodepop")
			sp++
			pc++
		} else if op == opcodeTailCall {
			fun := stack[sp]
			argc := ops[pc+1]
			if fun.primitive != nil {
				nextSp := sp + argc
				val, err := vm.callPrimitive(fun.primitive, stack[sp+1:nextSp+1])
				if err != nil {
					ops, pc, sp, env, err = vm.catch(err, stack, env)
					if err != nil {
						return nil, err
					}
				}
				stack[nextSp] = val
				sp = nextSp
				ops = env.ops
				pc = env.pc
				env = env.previous
				if env == nil {
					return stack[sp], nil
				}
			} else if fun.Type == fun.Type {
				ops, pc, sp, env, err = vm.tailcall(fun, argc, ops, stack, sp+1, env)
				if err != nil {
					return nil, err
				}
				if env == nil {
					return stack[sp], nil
				}
			} else if fun.Type == KeywordType {
				ops, pc, sp, env, err = vm.keywordTailcall(fun, argc, ops, stack, sp+1, env)
				if err != nil {
					ops, pc, sp, env, err = vm.catch(err, stack, env)
					if err != nil {
						return nil, err
					}
				} else {
					if env == nil {
						return stack[sp], nil
					}
				}
			} else {
				ops, pc, sp, env, err = vm.catch(Error(ArgumentErrorKey, "Not callable: ", fun), stack, env)
				if err != nil {
					return nil, err
				}
			}
		} else if op == opcodeLiteral {
//			println("opcodeLiteral")
			sp--
			stack[sp] = constants[ops[pc+1]]
			pc += 2
		} else if op == opcodeSetLocal {
			tmpEnv := env
			i := ops[pc+1]
			for i > 0 {
				tmpEnv = tmpEnv.locals
				i--
			}
			j := ops[pc+2]
			tmpEnv.elements[j] = stack[sp]
			pc += 3
		} else if op == opcodeClosure {
			sp--
			stack[sp] = Closure(constants[ops[pc+1]].code, env)
			pc = pc + 2
		} else if op == opcodeReturn {
			if env.previous == nil {
				return stack[sp], nil
			}
			ops = env.ops
			pc = env.pc
			env = env.previous
		} else if op == opcodeJump {
			pc += ops[pc+1]
		} else if op == opcodeDefGlobal {
			sym := constants[ops[pc+1]]
			defGlobal(sym, stack[sp])
			pc += 2
		} else if op == opcodeUndefGlobal {
			sym := constants[ops[pc+1]]
			undefGlobal(sym)
			pc += 2
		} else if op == opcodeDefMacro {
			sym := constants[ops[pc+1]]
			defMacro(sym, stack[sp])
			stack[sp] = sym
			pc += 2
		} else if op == opcodeImport {
			sym := constants[ops[pc+1]]
			err := Import(sym)
			if err != nil {
				ops, pc, sp, env, err = vm.catch(err, stack, env)
				if err != nil {
					return nil, err
				}
			} else {
				sp--
				stack[sp] = sym
				pc += 2
			}
		} else if op == opcodeVector {
			vlen := ops[pc+1]
			v := Vector(stack[sp : sp+vlen]...)
			sp = sp + vlen - 1
			stack[sp] = v
			pc += 2
		} else if op == opcodeStruct {
			vlen := ops[pc+1]
			v, _ := Struct(stack[sp : sp+vlen])
			sp = sp + vlen - 1
			stack[sp] = v
			pc += 2
		} else {
			panic("Bad instruction")
		}
	}
}

const stackColumn = 40 // Vile stack columns => 40

func showInstruction(pc int, op int, args string, stack []*Object, sp int) { // showInstruction function
	var body string
	body = leftJustified(fmt.Sprintf("%d ", pc), 8) + leftJustified(opsyms[op].text, 10) + args
	println(leftJustified(body, stackColumn), showStack(stack, sp))
}

func leftJustified(s string, width int) string {
	padsize := width - len(s)
	for i := 0; i < padsize; i++ {
		s += " "
	}
	return s
}

func truncatedObjectString(s string, limit int) string {
	if len(s) > limit {
		s = s[:limit]
		firstN := s[:limit-3]
		for i := limit - 1; i >= 0; i-- {
			if isWhitespace(s[i]) {
				s = s[:i]
				break
			}
		}
		if s == "" {
			s = firstN + "..."
		} else {
			openParens := 0
			for _, c := range s {
				switch c {
				case '(':
					openParens++
				case ')':
					openParens--
				}
			}
			if openParens > 0 {
				s += " ..."
				for i := 0; i < openParens; i++ {
					s += ")"
				}
			} else {
				s += "..."
			}
		}
	}
	return s
}
func showStack(stack []*Object, sp int) string {
	end := len(stack)
	s := "["
	limit := 5
	tail := ""
	if end-sp > limit {
		end = sp + limit
		tail = " ... "
	}
	for sp < end {
		tmp := fmt.Sprintf(" %v", Write(stack[sp]))
		s = s + truncatedObjectString(tmp, 30)
		sp++
	}
	return s + tail + " ]"
}

// used in exec when optimize||verbose||trace value is false
func (vm *vm) instrumentedExec(code *Code, env *frame) (*Object, error) {
	stack := make([]*Object, vm.stackSize) // stack
	sp := vm.stackSize // stack pointer
	ops := code.ops // opcodes
	pc := 0 // program counter
	var err error // error
	for {
		op := ops[pc]
		if op == opcodeCall { // CALL
//			println("call")
			if trace {
				showInstruction(pc, op, fmt.Sprintf("%d", ops[pc+1]), stack, sp)
			}
			argc := ops[pc+1]
			fun := stack[sp]
			if fun.primitive != nil {
				nextSp := sp + argc
				val, err := vm.callPrimitive(fun.primitive, stack[sp+1:nextSp+1])
				if err != nil {
					ops, pc, sp, env, err = vm.catch(err, stack, env)
					if err != nil {
						return nil, err
					}
				} else {
					stack[nextSp] = val
					sp = nextSp
					pc += 2
				}
			} else if fun.Type == FunctionType {
				ops, pc, sp, env, err = vm.funcall(fun, argc, ops, pc+2, stack, sp+1, env)
				if err != nil {
					return nil, err
				}
			} else if fun.Type == KeywordType {
				pc, sp, err = vm.keywordCall(fun, argc, pc+2, stack, sp+1)
				if err != nil {
					ops, pc, sp, env, err = vm.catch(err, stack, env)
					if err != nil {
						return nil, err
					}
				}
			} else {
				err := Error(ArgumentErrorKey, "Not callable: ", fun)
				ops, pc, sp, env, err = vm.catch(err, stack, env)
				if err != nil {
					return nil, err
				}
			}
		} else if op == opcodeGlobal { // GObjectAL
			sym := constants[ops[pc+1]]
			if sym.car == nil {
//				fmt.Printf("runtime.go checkpoint opcodeGlobal: %v and %T", sym.car)
				err := Error(ErrorKey, "Undefined symbol: ", sym)
				ops, pc, sp, env, err = vm.catch(err, stack, env)
				if err != nil {
					return nil, err
				}
			} else {
				if trace {
					showInstruction(pc, op, sym.text, stack, sp)
				}
				sp--
				stack[sp] = sym.car
				pc += 2
			}
		} else if op == opcodeLocal {
			if trace {
				showInstruction(pc, op, fmt.Sprintf("%d, %d", ops[pc+1], ops[pc+2]), stack, sp)
			}
			tmpEnv := env
			i := ops[pc+1]
			for i > 0 {
				tmpEnv = tmpEnv.locals
				i--
			}
			j := ops[pc+2]
			val := tmpEnv.elements[j]
//			fmt.Printf("\n\n opcodeLocal runtime.go checkpoint val: %v and %T \n\n", val, val)
			sp--
			stack[sp] = val
			pc += 3
		} else if op == opcodeJumpFalse {
			if trace {
				showInstruction(pc, op, fmt.Sprintf("%d", pc+ops[pc+1]), stack, sp)
			}
			b := stack[sp]
			sp++
			if b == False {
				pc += ops[pc+1]
			} else {
				pc += 2
			}
		} else if op == opcodePop {
			if trace {
				showInstruction(pc, op, "", stack, sp)
			}
			sp++
			pc++
		} else if op == opcodeTailCall {
			if interrupted || checkInterrupt() {
				return nil, addContext(env, Error(InterruptKey)) // not catchable
			}
			if trace {
				showInstruction(pc, op, fmt.Sprintf("%d", ops[pc+1]), stack, sp)
			}
			fun := stack[sp]
			argc := ops[pc+1]
			if fun.primitive != nil {
				nextSp := sp + argc
				val, err := vm.callPrimitive(fun.primitive, stack[sp+1:nextSp+1])
				if err != nil {
					ops, pc, sp, env, err = vm.catch(err, stack, env)
					if err != nil {
						return nil, err
					}
				} else {
					stack[nextSp] = val
					sp = nextSp
					ops = env.ops
					pc = env.pc
					env = env.previous
					if env == nil {
						return stack[sp], nil
					}
				}
			} else if fun.Type == FunctionType {
				ops, pc, sp, env, err = vm.tailcall(fun, argc, ops, stack, sp+1, env)
				if err != nil {
					return nil, err
				}
				if env == nil {
					return stack[sp], nil
				}
			} else if fun.Type == KeywordType {
				ops, pc, sp, env, err = vm.keywordTailcall(fun, argc, ops, stack, sp+1, env)
				if err != nil {
					return nil, err
				}
				if env.previous == nil {
					return stack[sp], nil
				}
			} else {
				return nil, addContext(env, Error(ArgumentErrorKey, "Not callable: ", fun))
			}
		} else if op == opcodeLiteral {
			if trace {
				showInstruction(pc, op, Write(constants[ops[pc+1]].Type), stack, sp)
			}
			sp--
			stack[sp] = constants[ops[pc+1]]
			pc += 2
		} else if op == opcodeSetLocal {
			if trace {
				showInstruction(pc, op, fmt.Sprintf("%d, %d", ops[pc+1], ops[pc+2]), stack, sp)
			}
			tmpEnv := env
			i := ops[pc+1]
			for i > 0 {
				tmpEnv = tmpEnv.locals
				i--
			}
			j := ops[pc+2]
			tmpEnv.elements[j] = stack[sp]
			pc += 3
		} else if op == opcodeClosure {
			if trace {
				showInstruction(pc, op, "", stack, sp)
			}
			sp--
			stack[sp] = Closure(constants[ops[pc+1]].code, env)
			pc = pc + 2
		} else if op == opcodeReturn {
			if interrupted || checkInterrupt() {
				return nil, addContext(env, Error(InterruptKey)) // not catchable
			}
			if trace {
				showInstruction(pc, op, "", stack, sp)
			}
			if env.previous == nil {
				return stack[sp], nil
			}
			ops = env.ops
			pc = env.pc
			env = env.previous
		} else if op == opcodeJump {
			if trace {
				showInstruction(pc, op, fmt.Sprintf("%d", pc+ops[pc+1]), stack, sp)
			}
			pc += ops[pc+1]
		} else if op == opcodeDefGlobal {
			sym := constants[ops[pc+1]]
			if trace {
				showInstruction(pc, op, sym.text, stack, sp)
			}
			defGlobal(sym, stack[sp])
			pc += 2
		} else if op == opcodeUndefGlobal {
			sym := constants[ops[pc+1]]
			if trace {
				showInstruction(pc, op, sym.text, stack, sp)
			}
			undefGlobal(sym)
			pc += 2
		} else if op == opcodeDefMacro {
			sym := constants[ops[pc+1]]
			if trace {
				showInstruction(pc, op, sym.text, stack, sp)
			}
			defMacro(sym, stack[sp])
			stack[sp] = sym
			pc += 2
		} else if op == opcodeImport {
			sym := constants[ops[pc+1]]
			if trace {
				showInstruction(pc, op, sym.text, stack, sp)
			}
			err := Import(sym)
			if err != nil {
				ops, pc, sp, env, err = vm.catch(err, stack, env)
				if err != nil {
					return nil, err
				}
			}
			sp--
			stack[sp] = sym
			pc += 2
		} else if op == opcodeVector {
			if trace {
				showInstruction(pc, op, fmt.Sprintf("%d", ops[pc+1]), stack, sp)
			}
			vlen := ops[pc+1]
			v := Vector(stack[sp : sp+vlen]...)
			sp = sp + vlen - 1
			stack[sp] = v
			pc += 2
		} else if op == opcodeStruct {
			if trace {
				showInstruction(pc, op, fmt.Sprintf("%d", ops[pc+1]), stack, sp)
			}
			vlen := ops[pc+1]
			v, _ := Struct(stack[sp : sp+vlen])
			sp = sp + vlen - 1
			stack[sp] = v
			pc += 2
		} else {
			panic("Bad instruction")
		}
	}
}
