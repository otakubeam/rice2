// Generated automatically.  DO NOT HAND-EDIT.

package adm3a

import "maunium.net/go/tcell/terminfo"

func init() {

	// lsi adm3a
	terminfo.AddTerminfo(&terminfo.Terminfo{
		Name:        "adm3a",
		Columns:     80,
		Lines:       24,
		Bell:        "\a",
		Clear:       "\x1a$<1/>",
		Italic:      "\x1b[3m",
		Strike:      "\x1b[9m",
		PadChar:     "\x00",
		SetCursor:   "\x1b=%p1%' '%+%c%p2%' '%+%c",
		CursorBack1: "\b",
		CursorUp1:   "\v",
		KeyUp:       "\v",
		KeyDown:     "\n",
		KeyRight:    "\f",
		KeyLeft:     "\b",
	})
}
