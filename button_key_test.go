package tbb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBtnKey(t *testing.T) {
	const k BtnKey = "tuAmt"

	assert.Equal(t, "tuAmt", k.String())
	assert.Equal(t, "\ftuAmt", k.Endpoint())

	b := k.Build("$5", "ton", "500")
	assert.Equal(t, Btn{Unique: "tuAmt", Text: "$5", Data: "ton|500"}, b)
	assert.Equal(t, Row{b}, k.AsRow("$5", "ton", "500"))
}
