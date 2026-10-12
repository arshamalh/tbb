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
