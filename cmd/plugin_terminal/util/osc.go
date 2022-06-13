package termutil

import (
	"fmt"
	"io"
)

func (t *Terminal) handleOSC() (renderRequired, exit bool) {

	params := []string{}
	param := ""

READ:
	for {
		r, _, err := t.reader.ReadRune()
		if err == io.EOF {
			return false, true
		}
		if t.isOSCTerminator(r) {
			params = append(params, param)
			break READ
		}
		if r == ';' {
			params = append(params, param)
			param = ""
			continue
		}
		param = fmt.Sprintf("%s%c", param, r)
	}

	if len(params) == 0 {
		return false, false
	}

	pT := params[len(params)-1]
	pS := params[:len(params)-1]

	if len(pS) == 0 {
		pS = []string{pT}
		pT = ""
	}

	switch pS[0] {
	case "0", "2", "l":
		t.setTitle(pT)
	case "10": // get/set foreground colour
		if len(pS) > 1 {
			if pS[1] == "?" {
				t.WriteToPty([]byte("\x1b]10;15"))
			}
		}
	case "11": // get/set background colour
		if len(pS) > 1 {
			if pS[1] == "?" {
				t.WriteToPty([]byte("\x1b]10;0"))
			}
		}
	}
	return false, false
}

func (t *Terminal) isOSCTerminator(r rune) bool {
	for _, terminator := range oscTerminators {
		if terminator == r {
			return true
		}
	}
	return false
}
