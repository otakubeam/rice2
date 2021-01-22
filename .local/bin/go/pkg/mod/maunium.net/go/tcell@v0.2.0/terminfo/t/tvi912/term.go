// Generated automatically.  DO NOT HAND-EDIT.

package tvi912

import "maunium.net/go/tcell/terminfo"

func init() {

	// old televideo 912/914/920
	terminfo.AddTerminfo(&terminfo.Terminfo{
		Name:         "tvi912",
		Aliases:      []string{"tvi914", "tvi920"},
		Columns:      80,
		Lines:        24,
		Bell:         "\a",
		Clear:        "\x1a",
		Underline:    "\x1bl",
		Italic:       "\x1b[3m",
		Strike:       "\x1b[9m",
		PadChar:      "\x00",
		SetCursor:    "\x1b=%p1%' '%+%c%p2%' '%+%c",
		CursorBack1:  "\b",
		CursorUp1:    "\v",
		KeyUp:        "\v",
		KeyDown:      "\n",
		KeyRight:     "\f",
		KeyLeft:      "\b",
		KeyBackspace: "\b",
		KeyHome:      "\x1e",
		KeyF1:        "\x01@\r",
		KeyF2:        "\x01A\r",
		KeyF3:        "\x01B\r",
		KeyF4:        "\x01C\r",
		KeyF5:        "\x01D\r",
		KeyF6:        "\x01E\r",
		KeyF7:        "\x01F\r",
		KeyF8:        "\x01G\r",
		KeyF9:        "\x01H\r",
	})
}
