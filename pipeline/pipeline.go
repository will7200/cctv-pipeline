package pipeline

import (
	"fmt"
	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
	"log"
)

// Pipeline wrapper around go-gst
type Pipeline struct {
	name     string
	elements []Element
	source   *Element
	pipeline *gst.Pipeline
}

func (p *Pipeline) Build() error {
	var err error
	p.pipeline, err = gst.NewPipeline(p.name)
	if err != nil {
		return err
	}
	for _, element := range p.elements {
		err = element.Build()
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Pipeline) Start(mainLoop *glib.MainLoop) error {
	var err error
	pipeline := p.pipeline
	// Start the pipeline
	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return err
	}

	// Add a message handler to the pipeline bus, logging interesting information to the console.
	pipeline.GetPipelineBus().AddWatch(func(msg *gst.Message) bool {
		switch msg.Type() {
		case gst.MessageEOS: // When end-of-stream is received stop the main loop
			pipeline.BlockSetState(gst.StateNull)
			mainLoop.Quit()
		case gst.MessageError: // Error messages are always fatal
			err := msg.ParseError()
			log.Println("ERROR:", err.Error())
			if debug := err.DebugString(); debug != "" {
				log.Println("DEBUG:", debug)
			}
			mainLoop.Quit()
		default:
			// All messages implement a Stringer. However, this is
			// typically an expensive thing to do and should be avoided.
			log.Println(msg.String())
		}
		return true
	})

	// Block on the main loop
	return mainLoop.RunError()
}

func NewIngestPipelineFromSource(name string, source *Element) (*Pipeline, error) {
	//depay := Element{
	//	Factory: "rtph265depay",
	//	Name:    "depay",
	//}
	parser := Element{
		Factory: "h265parse",
		Name:    "parser",
	}
	decoder := Element{
		Factory: "avdec_h265",
		Name:    "decoder",
	}
	converter := Element{
		Factory: "videoconvert",
		Name:    "converter",
	}

	x264enc := Element{
		Factory: "x264enc",
		Name:    "enc",
		Properties: map[string]interface{}{
			"speed-preset": "ultrafast",
			"pass":         "pass1",
		},
	}

	sink := Element{
		Factory: "splitmuxsink",
		Name:    "sink",
		Properties: map[string]interface{}{
			"target-duration": 1,
			"max-size-bytes":  uint64(10000),
			"async-finalize":  false,
			"location":        "data/segment%05d.ts",
		},
	}
	h264parse := Element{
		Factory: "h264parse",
		Name:    "parse",
	}
	pipeline := &Pipeline{
		name: name,
		elements: []Element{
			//depay,
			parser,
			decoder,
			converter,
			x264enc,
			h264parse,
			sink,
		},
		source: source,
	}

	if err := pipeline.Build(); err != nil {
		return nil, err
	}

	counter := 0
	_, err := sink.el.Connect("format-location", func(g *gst.Element) string {
		counter++
		name := fmt.Sprintf("segment%05d.ts", counter)
		log.Println("New file created:", name)
		return name
	})
	if err != nil {
		return nil, err
	}

	//depay.Link(&parser)
	parser.Link(&decoder)
	decoder.Link(&converter)
	converter.Link(&x264enc)
	x264enc.Link(&h264parse)
	h264parse.Link(&sink)

	return pipeline, nil
}
