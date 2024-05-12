package rtsp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReader(t *testing.T) {
	server := NewServerHandler(ServerHandlerParams{})
	assert.NotNil(t, server)
}
