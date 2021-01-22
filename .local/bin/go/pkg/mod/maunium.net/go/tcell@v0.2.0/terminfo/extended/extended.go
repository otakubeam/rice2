// Copyright 2019 The TCell Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package extended contains an extended set of terminal descriptions.
// Applications desiring to have a better chance of Just Working by
// default should include this package.  This will significantly increase
// the size of the program.
package extended

import (
	// The following imports just register themselves --
	// these are the terminal types we aggregate in this package.
	_ "maunium.net/go/tcell/terminfo/a/adm3a"
	_ "maunium.net/go/tcell/terminfo/a/aixterm"
	_ "maunium.net/go/tcell/terminfo/a/alacritty"
	_ "maunium.net/go/tcell/terminfo/a/ansi"
	_ "maunium.net/go/tcell/terminfo/a/aterm"
	_ "maunium.net/go/tcell/terminfo/b/beterm"
	_ "maunium.net/go/tcell/terminfo/b/bsdos_pc"
	_ "maunium.net/go/tcell/terminfo/c/cygwin"
	_ "maunium.net/go/tcell/terminfo/d/d200"
	_ "maunium.net/go/tcell/terminfo/d/d210"
	_ "maunium.net/go/tcell/terminfo/d/dtterm"
	_ "maunium.net/go/tcell/terminfo/e/emacs"
	_ "maunium.net/go/tcell/terminfo/e/eterm"
	_ "maunium.net/go/tcell/terminfo/g/gnome"
	_ "maunium.net/go/tcell/terminfo/h/hpterm"
	_ "maunium.net/go/tcell/terminfo/h/hz1500"
	_ "maunium.net/go/tcell/terminfo/k/konsole"
	_ "maunium.net/go/tcell/terminfo/k/kterm"
	_ "maunium.net/go/tcell/terminfo/l/linux"
	_ "maunium.net/go/tcell/terminfo/p/pcansi"
	_ "maunium.net/go/tcell/terminfo/r/rxvt"
	_ "maunium.net/go/tcell/terminfo/s/screen"
	_ "maunium.net/go/tcell/terminfo/s/simpleterm"
	_ "maunium.net/go/tcell/terminfo/s/sun"
	_ "maunium.net/go/tcell/terminfo/t/termite"
	_ "maunium.net/go/tcell/terminfo/t/tvi910"
	_ "maunium.net/go/tcell/terminfo/t/tvi912"
	_ "maunium.net/go/tcell/terminfo/t/tvi921"
	_ "maunium.net/go/tcell/terminfo/t/tvi925"
	_ "maunium.net/go/tcell/terminfo/t/tvi950"
	_ "maunium.net/go/tcell/terminfo/t/tvi970"
	_ "maunium.net/go/tcell/terminfo/v/vt100"
	_ "maunium.net/go/tcell/terminfo/v/vt102"
	_ "maunium.net/go/tcell/terminfo/v/vt220"
	_ "maunium.net/go/tcell/terminfo/v/vt320"
	_ "maunium.net/go/tcell/terminfo/v/vt400"
	_ "maunium.net/go/tcell/terminfo/v/vt420"
	_ "maunium.net/go/tcell/terminfo/v/vt52"
	_ "maunium.net/go/tcell/terminfo/w/wy50"
	_ "maunium.net/go/tcell/terminfo/w/wy60"
	_ "maunium.net/go/tcell/terminfo/w/wy99_ansi"
	_ "maunium.net/go/tcell/terminfo/x/xfce"
	_ "maunium.net/go/tcell/terminfo/x/xnuppc"
	_ "maunium.net/go/tcell/terminfo/x/xterm"
	_ "maunium.net/go/tcell/terminfo/x/xterm_kitty"
)
