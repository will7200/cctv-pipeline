package pipeline

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/rs/zerolog/log"
)

var _ PartialPipeline = (*SegmentationPipeline)(nil)

type SegmentationPipelineElements struct {
	converter *Element
	x265enc   *Element
	parser    *Element
	sink      *Element
}

type SegmentPipelineParams struct {
	// videoDuration is the amount of time of how long each segment should be
	videoDuration time.Duration
}

// SegmentationPipeline will split a given stream into smaller parts
type SegmentationPipeline struct {
	// elements
	Elements SegmentationPipelineElements
	// segment counter
	SegmentCounter uint32
	// File Segment Tracker
	fileSegmentTracker bool
	// parameters
	params SegmentPipelineParams
	// source element
	source *Element
}

// NewSegmentationPipeline create a new segmentation pipeline
func NewSegmentationPipeline(params SegmentPipelineParams) (sg *SegmentationPipeline, err error) {
	sg = new(SegmentationPipeline)
	sg.SegmentCounter = 0
	sg.params = params

	sg.Elements.converter = &Element{
		Factory: "videoconvert",
		Name:    "converter",
	}

	sg.Elements.x265enc = &Element{
		Factory: "x265enc",
		Name:    "enc",
		Properties: map[string]interface{}{
			"speed-preset": 1,
			"tune":         4,
			"key-int-max":  int(30),
		},
	}

	sg.Elements.parser = &Element{
		Factory: "h265parse",
		Name:    "",
	}

	sg.Elements.sink = &Element{
		Factory: "splitmuxsink",
		Name:    "sink",
		Properties: map[string]interface{}{
			"max-size-time":          uint64(params.videoDuration.Nanoseconds()),
			"async-finalize":         true,
			"location":               "segment%05d.ts",
			"muxer-factory":          "mpegtsmux",
			"send-keyframe-requests": true,
		},
	}

	return
}

func (sg *SegmentationPipeline) Prepare(pipeline *Pipeline) error {
	pipeline.AddWatch(sg.HandleNewSegment)
	spElements := []*Element{
		sg.Elements.converter,
		sg.Elements.parser,
		sg.Elements.sink,
		sg.Elements.x265enc,
	}
	pipeline.AddElements(spElements...)
	return nil
}

func (sg *SegmentationPipeline) HandleNewSegment(msg *gst.Message) bool {
	if msg.Source() != "sink" {
		return true
	}
	switch msg.Type() {
	case gst.MessageElement:
		structure := msg.GetStructure()
		// We get two events, one for the start and one for the end
		if val, err := structure.GetValue("location"); err == nil {
			if sg.fileSegmentTracker == false {
				sg.fileSegmentTracker = true
				return true
			} else {
				sg.fileSegmentTracker = false
			}
			runTime, _ := structure.GetValue("running-time")
			gRunTime := runTime.(uint64)
			log.Printf("New file created %s with current run-time of %v", val, gRunTime)
		}
	}
	return true
}

func (sg *SegmentationPipeline) HandleFormatLocation(g *gst.Element) string {
	sg.SegmentCounter++
	name := fmt.Sprintf("segment%05d.ts", sg.SegmentCounter)
	return name
}

func (sg *SegmentationPipeline) Build(pipeline *Pipeline) error {
	var err error

	links := []Link{
		{sg.Elements.converter, sg.Elements.x265enc},
		{sg.Elements.x265enc, sg.Elements.parser},
		{sg.Elements.parser, sg.Elements.sink},
	}
	for _, link := range links {
		if err := link.left.Link(link.right); err != nil {
			return err
		}
	}

	_, err = sg.Elements.sink.el.Connect("format-location", sg.HandleFormatLocation)
	if err != nil {
		return err
	}

	return nil
}

func (sg *SegmentationPipeline) Connect(source *Element) error {
	sg.source = source
	if _, err := sg.source.el.Connect("pad-added", func(src *gst.Element, pad *gst.Pad) {
		log.Printf("Pad '%s' has caps %s", pad.GetName(), pad.GetCurrentCaps().String())
		if pad.IsLinked() {
			log.Printf("Pad '%s' is already linked", pad.GetName())
			return
		}
		mediaType := pad.GetCurrentCaps().GetStructureAt(0).Name()
		if !strings.Contains(mediaType, "video/") {
			return
		}
		err := src.Link(sg.Elements.converter.el)
		if err != nil {
			log.Err(err)
		}
	}); err != nil {
		return err
	}
	return nil
}
