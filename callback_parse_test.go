package tbb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCallbackData(t *testing.T) {
	tests := []struct {
		data, unique, payload string
		ok                    bool
	}{
		{"\fbalance", "balance", "", true},
		{"\ftuAmt|ton|500", "tuAmt", "ton|500", true},
		{"\fsetLang|fa", "setLang", "fa", true},
		{"\fstk-pg_1|x", "stk-pg_1", "x", true},
		// Accepted now, rejected by the old regex (`(\|(.+))?$`):
		{"\fx|", "x", "", true},                         // empty payload after the separator
		{"\fx|line1\nline2", "x", "line1\nline2", true}, // multi-line payload
		// Not button data:
		{"", "", "", false},
		{"\f", "", "", false},
		{"\f|payload", "", "", false},
		{"balance", "", "", false},
		{"\fbad unique|x", "", "", false},
		{"\fünïcode|x", "", "", false},
	}
	for _, tt := range tests {
		unique, payload, ok := ParseCallbackData(tt.data)
		assert.Equal(t, tt.ok, ok, "%q", tt.data)
		assert.Equal(t, tt.unique, unique, "%q", tt.data)
		assert.Equal(t, tt.payload, payload, "%q", tt.data)
	}
}

func TestCallbackRoutingSplitsBeforeDispatch(t *testing.T) {
	b, err := NewBot(Settings{Offline: true, Synchronous: true})
	require.NoError(t, err)

	var got []Callback
	record := func(c Context) error { got = append(got, *c.Callback()); return nil }
	b.Handle("\fknown", record)
	b.Handle(OnCallback, record)

	b.ProcessUpdate(Update{Callback: &Callback{Data: "\fknown|a|b"}})
	b.ProcessUpdate(Update{Callback: &Callback{Data: "\funknown|x"}}) // no handler → OnCallback, still split
	b.ProcessUpdate(Update{Callback: &Callback{Data: "plain"}})       // not button data → untouched

	require.Len(t, got, 3)
	assert.Equal(t, Callback{Unique: "known", Data: "a|b", RawData: "\fknown|a|b"}, got[0])
	assert.Equal(t, Callback{Unique: "unknown", Data: "x", RawData: "\funknown|x"}, got[1])
	assert.Equal(t, Callback{Unique: "", Data: "plain", RawData: "plain"}, got[2])
}

func TestCallbackReprocessingDoesNotSplitTwice(t *testing.T) {
	b, err := NewBot(Settings{Offline: true, Synchronous: true})
	require.NoError(t, err)

	var payloads []string
	b.Handle("\fouter", func(c Context) error { payloads = append(payloads, c.Callback().Data); return nil })
	b.Handle("\finner", func(c Context) error { t.Fatal("payload was parsed as button data"); return nil })

	u := Update{Callback: &Callback{Data: "\fouter|\finner|x"}}
	b.ProcessUpdate(u)
	b.ProcessUpdate(u)
	assert.Equal(t, []string{"\finner|x", "\finner|x"}, payloads)
}
