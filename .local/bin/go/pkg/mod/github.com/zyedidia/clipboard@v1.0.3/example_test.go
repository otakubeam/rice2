package clipboard_test

import (
	"fmt"

	"github.com/zyedidia/clipboard"
)

func Example() {
	clipboard.WriteAll("日本語", "clipboard")
	text, _ := clipboard.ReadAll("clipboard")
	fmt.Println(text)

	// Output:
	// 日本語
}
