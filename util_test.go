package perf

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseGoIncr(t *testing.T) {
	assert.Equal(t, GoroutineIncr{}, ParseGoIncr(""))
	assert.Equal(t, GoroutineIncr{}, ParseGoIncr("0"))
	assert.Equal(t, GoroutineIncr{Up: 1, Dur: 10 * time.Second}, ParseGoIncr("1:10s"))
}
