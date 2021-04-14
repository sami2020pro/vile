package vile

// Intern - internalize the name into the global symbol table
func Intern(name string) *Object {
	sym, ok := symtab[name]
	if !ok {
		sym = new(Object)
		sym.text = name
		if IsValidKeywordName(name) {
			sym.Type = KeywordType
		} else if IsValidTypeName(name) {
			sym.Type = TypeType
		} else if IsValidSymbolName(name) {
			sym.Type = SymbolType
		} else {
			panic("invalid symbol/type/keyword name passed to intern: " + name)
		}
		symtab[name] = sym
	}
	return sym
}

func IsValidSymbolName(name string) bool {
	return len(name) > 0
}

func IsValidTypeName(s string) bool {
	n := len(s)
	if n > 2 && s[0] == '<' && s[n-1] == '>' {
		return true
	}
	return false
}

func IsValidKeywordName(s string) bool {
	n := len(s)
	if n > 1 && s[n-1] == ':' {
		return true
	}
	return false
}

func ToKeyword(obj *Object) (*Object, error) {
	switch obj.Type {
	case KeywordType:
		return obj, nil
	case TypeType:
		return Intern(obj.text[1:len(obj.text)-1] + ":"), nil
	case SymbolType:
		return Intern(obj.text + ":"), nil
	case StringType:
		if IsValidKeywordName(obj.text) {
			return Intern(obj.text), nil
		} else if IsValidSymbolName(obj.text) {
			return Intern(obj.text + ":"), nil
		}
	}
	return nil, Error(ArgumentErrorKey, "to-keyword expected a <keyword>, <type>, <symbol>, or <string>, got a ", obj.Type)
}

func typeNameString(s string) string {
	return s[1 : len(s)-1]
}

// <type> -> <symbol>
func TypeName(t *Object) (*Object, error) {
	if !IsType(t) {
		return nil, Error(ArgumentErrorKey, "type-name expected a <type>, got a ", t.Type)
	}
	return Intern(typeNameString(t.text)), nil
}

// <keyword> -> <symbol>
func KeywordName(t *Object) (*Object, error) {
	if !IsKeyword(t) {
		return nil, Error(ArgumentErrorKey, "keyword-name expected a <keyword>, got a ", t.Type)
	}
	return unkeyworded(t)
}

func keywordNameString(s string) string {
	return s[:len(s)-1]
}

func unkeywordedString(k *Object) string {
	if IsKeyword(k) {
		return keywordNameString(k.text)
	}
	return k.text
}

func unkeyworded(obj *Object) (*Object, error) {
	if IsSymbol(obj) {
		return obj, nil
	}
	if IsKeyword(obj) {
		return Intern(keywordNameString(obj.text)), nil
	}
	return nil, Error(ArgumentErrorKey, "Expected <keyword> or <symbol>, got ", obj.Type)
}

func ToSymbol(obj *Object) (*Object, error) {
	switch obj.Type {
	case KeywordType:
		return Intern(keywordNameString(obj.text)), nil
	case TypeType:
		return Intern(typeNameString(obj.text)), nil
	case SymbolType:
		return obj, nil
	case StringType:
		if IsValidSymbolName(obj.text) {
			return Intern(obj.text), nil
		}
	}
	return nil, Error(ArgumentErrorKey, "to-symbol expected a <keyword>, <type>, <symbol>, or <string>, got a ", obj.Type)
}

// the global symbol table. symbols for the basic types defined in this file are precached
var symtab = initSymbolTable() /* symtab is a map cause the returned value from initSymbolTable is a map */

func initSymbolTable() map[string]*Object {
	syms := make(map[string]*Object, 0)
	TypeType = &Object{text: "<type>"} // TypeType was defined in data.go
	TypeType.Type = TypeType // mutate to bootstrap type type
	syms[TypeType.text] = TypeType

	KeywordType = &Object{Type: TypeType, text: "<keyword>"} // KeywordType was defined in data.go
	syms[KeywordType.text] = KeywordType

	SymbolType = &Object{Type: TypeType, text: "<symbol>"} // SymbolType was defined in data.go
	syms[SymbolType.text] = SymbolType

	// println("checkpoint symbol.go - 146 line: ", syms[TypeType.text])

	return syms
}

// unknowns
func Symbols() []*Object {
	syms := make([]*Object, 0, len(symtab))
	for _, sym := range symtab {
		syms = append(syms, sym)
	}
	return syms
}

func Symbol(names []*Object) (*Object, error) {
	size := len(names)
	if size < 1 {
		return nil, Error(ArgumentErrorKey, "symbol expected at least 1 argument, got none")
	}
	name := ""
	for i := 0; i < size; i++ {
		o := names[i]
		s := ""
		switch o.Type {
		case StringType, SymbolType:
			s = o.text
		default:
			return nil, Error(ArgumentErrorKey, "symbol name component invalid: ", o)
		}
		name += s
	}
	return Intern(name), nil
}
