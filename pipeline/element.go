package pipeline

import (
	"github.com/go-gst/go-gst/gst"
)

// Element struct wraps a gst element
type Element struct {
	Factory    string
	Name       string
	Properties map[string]interface{}

	el *gst.Element
}

func (e *Element) Build() error {
	element, err := gst.NewElementWithName(e.Factory, e.Name)
	if err != nil {
		return err
	}
	if e.Properties != nil && len(e.Properties) > 0 {
		for k, v := range e.Properties {
			if err = element.Set(k, v); err != nil {
				return err
			}
		}
	}
	e.el = element
	return nil
}

func (e *Element) Link(other *Element) error {
	return e.el.Link(other.el)
}

func NewFileSrcElement(name string, file string) *Element {
	return &Element{
		Factory: "filesrc",
		Name:    name,
		Properties: map[string]interface{}{
			"location": file,
		},
		el: nil,
	}
}
