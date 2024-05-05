package pipeline

import (
	"fmt"
	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
	"log"
	"time"
)

// Link struct holds elements that should
// be linked left -> right
type Link struct {
	left  *Element
	right *Element
}

// Pipeline wrapper around go-gst
type Pipeline struct {
	name     string
	elements []*Element
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
	for index := range p.elements {
		err := p.pipeline.Add(p.elements[index].el)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Pipeline) Start(mainLoop *glib.MainLoop) error {
	var err error
	pipeline := p.pipeline

	// Add a message handler to the pipeline bus, logging interesting information to the console.
	pipeline.GetPipelineBus().AddWatch(func(msg *gst.Message) bool {
		switch msg.Type() {
		case gst.MessageEOS: // When end-of-stream is received stop the main loop
			log.Println("End of Stream")
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
	// Start the pipeline
	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return err
	}

	// Block on the main loop
	return mainLoop.RunError()
}

type SegmentPipeline struct {
	*Pipeline

	// segment counter
	SegmentCounter uint32
}

type SegmentPipelineParams struct {
	// videoDuration is the amount of time of how long each segment should be
	videoDuration time.Duration
}

func NewSegmentPipeline(name string, params SegmentPipelineParams, otherElements ...*Element) (sg *SegmentPipeline, err error) {
	converter := &Element{
		Factory: "videoconvert",
		Name:    "converter",
	}

	x264enc := &Element{
		Factory: "x264enc",
		Name:    "enc",
		Properties: map[string]interface{}{
			//"speed-preset": 1,
			"pass":        17,
			"tune":        uint(4),
			"key-int-max": uint(30),
		},
	}

	parser := &Element{
		Factory: "h264parse",
		Name:    "",
	}

	sink := &Element{
		Factory: "splitmuxsink",
		Name:    "sink",
		Properties: map[string]interface{}{
			// one second currently
			"max-size-time":  uint64(params.videoDuration.Nanoseconds()),
			"async-finalize": true,
			"location":       "segment%05d.ts",
			"muxer-factory":  "mpegtsmux",
		},
	}
	pipeline := &Pipeline{
		name: name,
		elements: append([]*Element{
			converter,
			x264enc,
			parser,
			sink,
		}, otherElements...),
		source: otherElements[0],
	}

	if err := pipeline.Build(); err != nil {
		return nil, err
	}

	if _, err := pipeline.source.el.Connect("pad-added", func(src *gst.Element, pad *gst.Pad) {
		log.Printf("Pad '%s' has caps %s", pad.GetName(), pad.GetCurrentCaps().String())
		if pad.IsLinked() {
			log.Printf("Pad '%s' is already linked", pad.GetName())
			return
		}

		err := src.Link(converter.el)
		if err != nil {
			log.Println(err)
		}
	}); err != nil {
		return nil, err
	}

	links := []Link{
		{converter, x264enc},
		{x264enc, parser},
		{parser, sink},
	}
	for index := 1; index < len(otherElements); index++ {
		links = append(links, Link{otherElements[index], otherElements[index-1]})
	}
	for _, link := range links {
		if err := link.left.Link(link.right); err != nil {
			return nil, err
		}
	}

	sg = new(SegmentPipeline)
	sg.Pipeline = pipeline
	sg.SegmentCounter = 0

	_, err = sink.el.Connect("format-location", func(g *gst.Element) string {
		sg.SegmentCounter++
		name := fmt.Sprintf("segment%05d.ts", sg.SegmentCounter)
		log.Println("New file created:", name)
		return name
	})
	if err != nil {
		return nil, err
	}

	return
}
