package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestElement(t *testing.T) {
	element := NewFileSrcElement("src", "data")
	assert.Equal(t, element.Name, "src")
	assert.Equal(t, element.Factory, "filesrc")
}
