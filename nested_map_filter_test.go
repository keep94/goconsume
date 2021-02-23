package goconsume

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNestedMapFilter(t *testing.T) {
	assert := assert.New(t)
	var result []string
	c := MapFilter(AppendTo(&result), appendStr("d"), appendStr("e"))
	c = MapFilter(c, appendStr("a"), appendStr("b"), appendStr("c"))
	if c.CanConsume() {
		str := ""
		c.Consume(&str)
	}
	assert.Equal([]string{"abcde"}, result)
	mpc := c.(*mapFilterConsumer)
	assert.IsType((*appendConsumer)(nil), mpc.Consumer)
}

func appendStr(s string) func(src, dest *string) bool {
	return func(src, dest *string) bool {
		*dest = *src + s
		return true
	}
}
