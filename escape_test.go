package botbase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeMarkdownV2Basics(t *testing.T) {
	assert.Equal(t,
		`*Balance:* $1\.50 \(approx\.\) \- `+"`code`"+` _it_\!`,
		EscapeMarkdownV2Basics("*Balance:* $1.50 (approx.) - `code` _it_!"),
	)
}

func TestEscapeMarkdownV2(t *testing.T) {
	assert.Equal(t, `john\_doe \*x\* \\ 1\.0`, EscapeMarkdownV2(`john_doe *x* \ 1.0`))
}

func TestEscapeMarkdownV2Code(t *testing.T) {
	// Inside code only ` and \ are special; dots and dashes stay as they are.
	assert.Equal(t, "docker login reg.example.com -u a\\`b \\\\", EscapeMarkdownV2Code("docker login reg.example.com -u a`b \\"))
}

func TestEscapeMarkdownV2URL(t *testing.T) {
	assert.Equal(t, `https://x.dev/a_(b\)?q=1.2`, EscapeMarkdownV2URL("https://x.dev/a_(b)?q=1.2"))
}

func TestEscapeLegacyMarkdownAndHTML(t *testing.T) {
	assert.Equal(t, `john\_doe \*x\* \[a] `+"\\`c\\`", EscapeMarkdown("john_doe *x* [a] `c`"))
	assert.Equal(t, `a &lt;b&gt; &amp; &quot;c&quot; 'd'`, EscapeHTML(`a <b> & "c" 'd'`))
}

func TestEscapeByMode(t *testing.T) {
	in := "a_b <c>."
	assert.Equal(t, `a\_b \<c\>\.`, Escape(ModeMarkdownV2, in))
	assert.Equal(t, `a\_b <c>.`, Escape(ModeMarkdown, in))
	assert.Equal(t, `a_b &lt;c&gt;.`, Escape(ModeHTML, in))
	assert.Equal(t, in, Escape(ModeDefault, in))
}
