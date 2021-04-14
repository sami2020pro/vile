package repl

import (
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

// // darwin
// var getTermios = syscall.TIOCGETA
// var setTermios = syscall.TIOCSETA

// linux
var getTermios = syscall.TCGETS
var setTermios = syscall.TCSETS

type ReplHandler interface {
	Eval(expr string) (string, bool, error)
	Complete(expr string) (string, []string)
	Reset()
	Prompt() string
	Start() []string
	Stop(history []string)
}

var input chan byte
var lastIn byte
var lastInOk bool
var state *termState

func REPL(handler ReplHandler) error {
	var err error
	input = make(chan byte, 1)
	go func() {
		var ch [1]byte
		for {
			n, err := syscall.Read(syscall.Stdin, ch[:])
			if err != nil || n == 0 {
				panic("Problem reading stdin")
			} else {
				input <- ch[0]
				if ch[0] == 0 {
					return
				}
			}
		}
	}()
	state, err = MakeCbreak(syscall.Stdin)
	if err == nil {
		defer Restore(syscall.Stdin, state)
		err = repl(handler)
		return err
	} else {
		return err
	}
}

func Exit(code int) {
	if state != nil {
		Restore(syscall.Stdin, state)
		black := "\033[0;0m"
		fmt.Printf(black)
	}
	os.Exit(1)
}

func GetChar() byte {
	if lastInOk {
		lastInOk = false
		return lastIn
	}
	return <-input
}

func Pause(millis time.Duration) {
	if !lastInOk {
		select {
		case ch := <-input:
			lastIn = ch
			lastInOk = true
		case <-time.After(millis):
		}
	}
}

func PutChar(b byte) error {
	var ch [1]byte
	ch[0] = b
	_, err := syscall.Write(syscall.Stdout, ch[:])
	return err
}

func PutChars(b []byte) error {
	_, err := syscall.Write(syscall.Stdout, b)
	return err
}

func PeekChar() (byte, bool) {
	if lastInOk {
		return lastIn, true
	}
	select {
	case ch := <-input:
		lastIn = ch
		lastInOk = true
		return lastIn, true
	case <-time.After(10 * time.Millisecond):
		return 0, false
	}
}

// State contains the state of a terminal.
type termState struct {
	termios syscall.Termios
}

// MakeRaw put the terminal connected to the given file descriptor into raw
// mode and returns the previous state of the terminal so that it can be
// restored.
func MakeRaw(fd int) (*termState, error) {
	var oldState termState
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(getTermios), uintptr(unsafe.Pointer(&oldState.termios)), 0, 0, 0); err != 0 {
		return nil, err
	}

	newState := oldState.termios
	newState.Iflag &^= syscall.ISTRIP | syscall.INLCR | syscall.ICRNL | syscall.IGNCR | syscall.IXON | syscall.IXOFF
	newState.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(setTermios), uintptr(unsafe.Pointer(&newState)), 0, 0, 0); err != 0 {
		return nil, err
	}

	return &oldState, nil
}

func MakeCbreak(fd int) (*termState, error) {
	var oldState termState
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(getTermios), uintptr(unsafe.Pointer(&oldState.termios)), 0, 0, 0); err != 0 {
		return nil, err
	}

	newState := oldState.termios
	newState.Iflag &^= syscall.ISTRIP | syscall.INLCR | syscall.ICRNL | syscall.IGNCR | syscall.IXON | syscall.IXOFF
	newState.Lflag &^= syscall.ECHO | syscall.ICANON
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(setTermios), uintptr(unsafe.Pointer(&newState)), 0, 0, 0); err != 0 {
		return nil, err
	}

	return &oldState, nil
}

// Restore restores the terminal connected to the given file descriptor to a
// previous state.
func Restore(fd int, state *termState) error {
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(setTermios), uintptr(unsafe.Pointer(&state.termios)), 0, 0, 0)
	return err
}

func PutString(s string) error {
	return PutChars([]byte(s))
}

func cursorBackward() error {
	chars := []byte{27, '[', '1', 'D'}
	return PutChars(chars)
}

func cursorForward() error {
	chars := []byte{27, '[', '1', 'C'}
	return PutChars(chars)
}

type lineBuf struct {
	length       int
	cursor       int
	buf          []byte
	yanked       string
	yanking      bool
	history      []string
	historyIndex int
}

func newLineBuf(capacity int) *lineBuf {
	storage := make([]byte, capacity)
	lb := lineBuf{0, 0, storage[:], "", false, nil, -1}
	return &lb
}

func (lb *lineBuf) IsEmpty() bool {
	return lb.length == 0
}

func (lb *lineBuf) Clear() {
	lb.length = 0
	lb.cursor = 0
	lb.yanking = false
}

func (lb *lineBuf) Insert(ch byte) {
	lb.yanking = false
	n := len(lb.buf)
	if lb.length == n {
		target := make([]byte, n+10)
		copy(target, lb.buf[:n])
		lb.buf = target
	}
	if lb.cursor == lb.length {
		lb.buf[lb.cursor] = ch
	} else {
		copy(lb.buf[lb.cursor+1:], lb.buf[lb.cursor:])
		lb.buf[lb.cursor] = ch
	}
	lb.cursor = lb.cursor + 1
	lb.length = lb.length + 1
}

func (lb *lineBuf) InsertBytes(chs []byte) {
	for _, ch := range chs {
		lb.Insert(ch)
	}
}

func (lb *lineBuf) Delete() bool {
	lb.yanking = false
	if lb.cursor < lb.length {
		copy(lb.buf[lb.cursor:], lb.buf[lb.cursor+1:])
		lb.length = lb.length - 1
		return true
	} else {
		return false
	}
}

func (lb *lineBuf) KillToEnd() int {
	n := lb.length - lb.cursor
	//for now, a single yank buffer, not a stack
	if lb.yanking {
		lb.yanked = lb.yanked + string(lb.buf[lb.cursor:lb.length])
	} else {
		lb.yanked = string(lb.buf[lb.cursor:lb.length])
	}
	lb.length = lb.cursor
	lb.yanking = false
	return n
}

func (lb *lineBuf) DeleteRange(begin int, end int) int {
	if begin < 0 {
		begin = 0
	} else if begin > lb.length {
		return 0
	}
	if end > lb.length {
		end = lb.length
	} else if end < 0 {
		return 0
	}
	n := end - begin
	if n > 0 {
		if lb.yanking {
			lb.yanked = lb.yanked + string(lb.buf[begin:end])
		} else {
			lb.yanked = string(lb.buf[begin:end])
		}
		copy(lb.buf[begin:], lb.buf[end:])
		lb.length = lb.length - n
		lb.cursor = begin
	}
	return n
}

func isWordDelimiter(ch byte) bool {
	if ch == SPACE || ch == OPEN_PAREN || ch == OPEN_BRACKET || ch == OPEN_BRACE || ch == SINGLE_QUOTE {
		return true
	}
	return false
}

func (lb *lineBuf) previousWordBoundary() int {
	i := lb.cursor
	if i == 0 {
		return 0
	} else {
		i--
		if i == 0 {
			return 0
		}
		for isWordDelimiter(lb.buf[i]) {
			i--
			if i < 0 {
				return 0
			}
		}
		if i > 0 {
			for !isWordDelimiter(lb.buf[i]) {
				i--
				if i < 0 {
					return 0
				}
			}
		}
		return i + 1
	}
}

func (lb *lineBuf) WordBackspace() int {
	i := lb.previousWordBoundary()
	return lb.DeleteRange(i, lb.cursor)
}

func (lb *lineBuf) WordDelete() int {
	var i int
	for i = lb.cursor - 1; i < lb.length; i++ {
		if lb.buf[i] != SPACE {
			break
		}
	}
	for ; i < lb.length; i++ {
		if lb.buf[i] == SPACE {
			return lb.DeleteRange(lb.cursor, i)
		}
	}
	return 0
}

func (lb *lineBuf) WordForward() {
	i := lb.cursor
	for ; i < lb.length; i++ {
		if lb.buf[i] != SPACE {
			break
		}
	}
	for ; i < lb.length; i++ {
		if lb.buf[i] == SPACE {
			lb.cursor = i
			return
		}
	}
	lb.cursor = lb.length
}

func (lb *lineBuf) WordBackward() {
	lb.cursor = lb.previousWordBoundary()
}

func (lb *lineBuf) Yank() int {
	lb.yanking = true
	lb.InsertBytes([]byte(lb.yanked))
	return len(lb.yanked)

}

func (lb *lineBuf) Backward() bool {
	lb.yanking = false
	if lb.cursor > 0 {
		lb.cursor = lb.cursor - 1
		return true
	} else {
		return false
	}
}

func (lb *lineBuf) Forward() bool {
	lb.yanking = false
	if lb.cursor < lb.length {
		lb.cursor = lb.cursor + 1
		return true
	} else {
		return false
	}
}

func (lb *lineBuf) Begin() {
	lb.yanking = false
	lb.cursor = 0
}

func (lb *lineBuf) End() {
	lb.yanking = false
	lb.cursor = lb.length
}

func (lb *lineBuf) AddToHistory(line string) {
	if len(line) > 0 {
		lb.history = append(lb.history, line)
	}
	lb.historyIndex = -1
}

func (lb *lineBuf) PrevInHistory() int {
	n := lb.length
	if lb.history != nil {
		if lb.historyIndex < 0 {
			lb.historyIndex = len(lb.history) - 1
		} else {
			lb.historyIndex--
		}
		if lb.historyIndex >= 0 {
			lb.length = 0
			lb.cursor = 0
			lb.InsertBytes([]byte(lb.history[lb.historyIndex]))
			if lb.length > n {
				n = lb.length
			}
		} else {
			lb.historyIndex = 0
		}
	}
	return n
}

func (lb *lineBuf) NextInHistory() int {
	n := lb.length
	if lb.history != nil {
		if lb.historyIndex >= 0 {
			lb.historyIndex++
			if lb.historyIndex < len(lb.history) {
				lb.length = 0
				lb.cursor = 0
				lb.InsertBytes([]byte(lb.history[lb.historyIndex]))
				if lb.length > n {
					n = lb.length
				}
			} else {
				lb.historyIndex--
			}
		}
	}
	return n
}

func (lb *lineBuf) String() string {
	return string(lb.buf[0:lb.length])
}

const CTRL_A = 1
const CTRL_B = 2
const CTRL_C = 3
const CTRL_D = 4
const CTRL_E = 5
const CTRL_F = 6
const BEEP = 7
const BACKSPACE = 8
const TAB = 9
const NEWLINE = 10
const CTRL_K = 11
const CTRL_L = 12
const RETURN = 13
const CTRL_N = 14
const CTRL_P = 16
const CTRL_Y = 25
const ESCAPE = 27
const SPACE = 32
const SINGLE_QUOTE = 39
const DELETE = 127
const OPEN_PAREN = 40
const CLOSE_PAREN = 41
const OPEN_BRACKET = 91
const CLOSE_BRACKET = 93
const OPEN_BRACE = 123
const CLOSE_BRACE = 125

func matching(ch byte) byte {
	switch ch {
	case CLOSE_PAREN:
		return OPEN_PAREN
	case CLOSE_BRACKET:
		return OPEN_BRACKET
	case CLOSE_BRACE:
		return OPEN_BRACE
	default:
		return 0
	}
}

func highlightMatch(lb *lineBuf, prompt string, chOpen byte, chClose byte) {
	var i = lb.cursor - 1
	count := 1
	for i > 0 {
		i--
		if lb.buf[i] == chOpen {
			count--
			if count == 0 {
				tmp := lb.cursor
				lb.cursor = i
				drawline(prompt, lb, 0)
				Pause(500 * time.Millisecond)
				lb.cursor = tmp
				drawline(prompt, lb, 0)
				return
			}
		} else if lb.buf[i] == chClose {
			count++
		}
	}
	PutChar(BEEP)
}

func dump(prompt string, lb lineBuf, extra int) {
	fmt.Println("\ncursor =", lb.cursor, "length =", lb.length)
	for i := 0; i < lb.length; i++ {
		PutChar(lb.buf[i])
	}
	PutChar(NEWLINE)
	for i := 0; i < lb.length; i++ {
		if i == lb.cursor {
			PutChar('^')
		} else {
			PutChar('.')
		}
	}
	if lb.cursor == lb.length {
		PutChar('^')
	}
	PutChar(NEWLINE)
}

func drawline(prompt string, lb *lineBuf, extra int) {
	PutChar(13)
	PutString(prompt)
	PutString(lb.String())
	for i := 0; i < extra; i++ {
		PutChar(SPACE)
	}
	cursor := lb.length + extra
	for cursor > lb.cursor {
		cursorBackward()
		cursor = cursor - 1
	}
}

func repl(handler ReplHandler) error {
	buf := newLineBuf(1024)
	hist := handler.Start()
	if hist != nil {
		buf.history = hist
	}
	prompt := handler.Prompt()
	PutString(prompt)
	meta := false
	metaExt := false
	var lastChar byte
	var options []string
	for true {
		ch := GetChar()
		if metaExt {
			metaExt = false
			switch ch {
			case 'D':
				if buf.Backward() {
					cursorBackward()
					drawline(prompt, buf, 0)
				}
			case 'C':
				if buf.Forward() {
					cursorForward()
					drawline(prompt, buf, 0)
				}
			case 'B':
				n := buf.NextInHistory()
				drawline(prompt, buf, n)
			case 'A':
				n := buf.PrevInHistory()
				drawline(prompt, buf, n)
			default:
				PutChar(BEEP)
			}
		} else if meta {
			meta = false
			switch ch {
			case DELETE:
				n := buf.WordBackspace()
				drawline(prompt, buf, n)
			case 'd':
				n := buf.WordDelete()
				drawline(prompt, buf, n)
			case 'b':
				buf.WordBackward()
				drawline(prompt, buf, 0)
			case 'f':
				buf.WordForward()
				drawline(prompt, buf, 0)
			case OPEN_BRACKET:
				metaExt = true
			default:
				PutChar(BEEP)
			}
		} else {
			switch ch {
			case ESCAPE:
				meta = true
			case CTRL_D:
				if buf.IsEmpty() {
					PutString("\n")
					handler.Stop(buf.history)
					input <- 0 //to stop the goroutine
					return nil
				} else {
					buf.Delete()
					drawline(prompt, buf, 1)
				}
			case CTRL_A:
				buf.Begin()
				drawline(prompt, buf, 0)
			case CTRL_E:
				buf.End()
				drawline(prompt, buf, 0)
			case CTRL_F:
				if buf.Forward() {
					cursorForward()
					drawline(prompt, buf, 0)
				}
			case CTRL_B:
				if buf.Backward() {
					cursorBackward()
					drawline(prompt, buf, 0)
				}
			case CTRL_C:
				PutString("*** Interrupt\n")
				buf.Clear()
				handler.Reset()
				prompt = handler.Prompt()
				PutString(prompt)
			case CTRL_K:
				n := buf.KillToEnd()
				drawline(prompt, buf, n)
			case CTRL_Y:
				n := buf.Yank()
				drawline(prompt, buf, n)
			case CTRL_L:
				//dump(prompt, buf, 0);
				PutString("\n")
				drawline(prompt, buf, 0)
			case CTRL_N:
				n := buf.NextInHistory()
				drawline(prompt, buf, n)
			case CTRL_P:
				n := buf.PrevInHistory()
				drawline(prompt, buf, n)
			case TAB:
				if _, ok := PeekChar(); ok {
					//pasting text in, don't do the tab completion
					ch = 0
				} else if lastChar == TAB {
					if options != nil {
						for _, opt := range options {
							PutChar(NEWLINE)
							PutString(opt)
						}
						PutChar(NEWLINE)
						drawline(prompt, buf, 0)
					}
					PutChar(BEEP)
				} else {
					addendum, opt := handler.Complete(string(buf.buf[0:buf.cursor]))
					if len(addendum) > 0 {
						buf.InsertBytes([]byte(addendum))
					}
					if len(opt) == 1 {
						buf.Insert(' ')
						options = nil
					} else {
						options = opt
						PutChar(BEEP)
					}
					drawline(prompt, buf, 0)
				}
			case DELETE:
				if buf.Backward() {
					buf.Delete()
					drawline(prompt, buf, 1)
				} else {
					PutChar(BEEP)
				}
			case RETURN:
				if !buf.IsEmpty() {
					PutChar('\n')
				}
				s := buf.String()
				buf.AddToHistory(s)
				buf.Clear()
				red := "\033[0;31m"
				green := "\033[0;32m"
				blue := "\033[0;34m"
				black := "\033[0;0m"
				fmt.Printf(blue) //all eval output in blue
				result, more, err := handler.Eval(s)
				fmt.Printf(black)
				if err != nil {
					fmt.Println(red, "***", err, black) //error result in red
					buf.Clear()
					prompt = handler.Prompt()
					PutString(prompt)
				} else if more {
					prompt = ""
				} else {
					fmt.Println(green + result + black) //non-error result in green
					prompt = handler.Prompt()
					PutString(prompt)
				}
			default:
				if ch >= SPACE && ch < 127 {
					buf.Insert(ch)
					drawline(prompt, buf, 0)
					match := matching(ch)
					if match != 0 {
						highlightMatch(buf, prompt, match, ch)
					}
				} else {
					PutChar(BEEP)
				}
			}
		}
		lastChar = ch

	}
	return nil //never happens
}
