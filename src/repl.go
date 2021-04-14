package vile

import (
	"errors"
	"io/ioutil"
	"os"
	"os/user"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"fmt"

	"github.com/sami2020pro/vile/src/repl"
)

type vileHandler struct {
	buf string
}

func (vile *vileHandler) Eval(expr string) (string, bool, error) {
	// return result, needMore, error
	for checkInterrupt() {
	} // to clear out any that happened while sitting in getc
	interrupted = false
	whole := strings.Trim(vile.buf+expr, " ")
	opens := len(strings.Split(whole, "("))
	closes := len(strings.Split(whole, ")"))
	if opens > closes {
		vile.buf = whole + " "
		return "", true, nil
	} else if closes > opens {
		vile.buf = ""
		return "", false, errors.New("unbalanced ')' encountered")
	} else {
		// this is the normal case
		if whole == "" {
			return "", false, nil
		}
		lexpr, err := Read(String(whole), AnyType)
		vile.buf = ""
		if err == nil {
			val, err := Eval(lexpr)
			if err == nil {
				result := ""
				if val == nil {
					result = " !!! whoops, result is nil, that isn't right"
					panic("here")
				} else {
					result = "= " + Write(val)
				}
				return result, false, nil
			}
			return "", false, err
		}
		return "", false, err
	}
}

func (vile *vileHandler) Reset() {
	vile.buf = ""
}

func greatestCommonPrefixLength(s1 string, s2 string) int {
	max := len(s1)
	l2 := len(s2)
	if l2 < max {
		max = l2
	}
	for i := 0; i < max; i++ {
		if s1[i] != s2[i] {
			return i - 1
		}
	}
	return max
}

func greatestCommonPrefix(matches []string) string {
	switch len(matches) {
	case 0:
		return ""
	case 1:
		return matches[0]
	default:
		s := matches[0]
		max := len(matches)
		greatest := len(s)
		for i := 1; i < max; i++ {
			n := greatestCommonPrefixLength(s, matches[i])
			if n < greatest {
				greatest = n
				s = s[:n+1]
			}
		}
		return s
	}
}

func (vile *vileHandler) completePrefix(expr string) (string, bool) {
	prefix := ""
	funPosition := false
	exprLen := len(expr)
	if exprLen > 0 {
		i := exprLen - 1
		ch := expr[i]
		if !isWhitespace(ch) && !isDelimiter(ch) {
			if i > 0 {
				i--
				for {
					ch = expr[i]
					if isWhitespace(ch) || isDelimiter(ch) {
						funPosition = ch == '('
						prefix = expr[i+1:]
						break
					}
					i--
					if i < 0 {
						prefix = expr
						break
					}
				}
			} else {
				prefix = expr
			}
		}
	}
	return prefix, funPosition
}

func (vile *vileHandler) Complete(expr string) (string, []string) {
	var matches []string
	addendum := ""
	prefix, funPosition := vile.completePrefix(expr)
	candidates := map[*Object]bool{}
	if funPosition {
		for _, sym := range GetKeywords() {
			str := sym.String()
			if strings.HasPrefix(str, prefix) {
				candidates[sym] = true
			}
		}
		for _, sym := range Macros() {
			_, ok := candidates[sym]
			if !ok {
				str := sym.String()
				if strings.HasPrefix(str, prefix) {
					candidates[sym] = true
				}
			}
		}
	}
	for _, sym := range Globals() {
		_, ok := candidates[sym]
		if !ok {
			_, ok := candidates[sym]
			if !ok {
				str := sym.String()
				if strings.HasPrefix(str, prefix) {
					if funPosition {
						val := GetGlobal(sym)
						if IsFunction(val) {
							candidates[sym] = true
						}
					} else {
						candidates[sym] = true
					}
				}
			}
		}
	}
	for sym := range candidates {
		matches = append(matches, sym.String())

	}
	sort.Strings(matches)
	gcp := greatestCommonPrefix(matches)
	if len(gcp) > len(prefix) {
		addendum = gcp[len(prefix):]
	}
	return addendum, matches
}

func (vile *vileHandler) Prompt() string {
	user, err := user.Current()
	if err != nil {
		panic(err)
	}

	prompt := GetGlobal(Intern("*prompt*"))
	if prompt != nil {
		return prompt.String()
	}

	return fmt.Sprintf("(\033[37m%v\033[0m)> ", user.Username)
}

func historyFileName() string {
	return filepath.Join(os.Getenv("HOME"), ".vile_history")
}

func (vile *vileHandler) Start() []string {
	content, err := ioutil.ReadFile(historyFileName())
	if err != nil {
		return nil
	}
	s := strings.Split(string(content), "\n")
	var s2 []string
	for _, v := range s {
		if v != "" {
			s2 = append(s2, v)
		}
	}
	return s2
}

func (vile *vileHandler) Stop(history []string) {
	if len(history) > 100 {
		history = history[len(history)-100:]
	}
	content := strings.Join(history, "\n") + "\n"
	err := ioutil.WriteFile(historyFileName(), []byte(content), 0644)
	if err != nil {
		println("[warning: cannot write ", historyFileName(), "]")
	}
}

func ReadEvalPrintLoop() {
	interrupts = make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)
	handler := vileHandler{""}
	err := repl.REPL(&handler)
	if err != nil {
		println("REPL error: ", err)
	}
}

func exit(code int) {
	Cleanup()
	repl.Exit(code)
}
