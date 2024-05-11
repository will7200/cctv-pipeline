package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	// segmentBasePath base file path
	segmentBasePath string
	// cameraId this camera
	cameraId string
	// ensureSegmentDuration checks that a segment has been produced in the given interval
	ensureSegmentDuration time.Duration
	// callback for when a new sample is available
	onNewSample func(sample *gst.Sample)
	// callback for new buffer
	onNewBuffer func(buffer *gst.Buffer)
	// callback for new source
	onNewSource func(caps *gst.Caps)
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
	// lastFinishedSegment is the time of the last finished segment
	lastFinishedSegment time.Time
	// holds the first frame available for the current segment
	frameBuffer *gst.Buffer
	// quit holds a chan to send to stop checking this part of the pipeline
	quit  chan struct{}
	mutex sync.RWMutex
}

// NewSegmentationPipeline create a new segmentation pipeline
func NewSegmentationPipeline(params SegmentPipelineParams) (sg *SegmentationPipeline, err error) {
	sg = new(SegmentationPipeline)
	sg.SegmentCounter = 0
	sg.params = params
	sg.lastFinishedSegment = time.Now()
	sg.mutex = sync.RWMutex{}

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
			sg.mutex.Lock()
			sg.lastFinishedSegment = time.Now()
			if sg.frameBuffer != nil && sg.params.onNewBuffer != nil {
				sg.params.onNewBuffer(sg.frameBuffer)
				sg.frameBuffer.Unref()
				sg.frameBuffer = nil
			}
			sg.mutex.Unlock()
		}
	}
	return true
}

func (sg *SegmentationPipeline) HandleFormatLocation(g *gst.Element, val uint, sample *gst.Sample) string {
	sg.SegmentCounter++
	current := time.Now().UTC()
	dir := fmt.Sprintf("%s/%s/%s",
		sg.params.segmentBasePath,
		sg.params.cameraId,
		current.Format("2006/01/02"),
	)
	err := os.MkdirAll(dir, os.FileMode(0744))
	if err != nil {
		log.Err(err).Msg("unable to create directory")
	}
	name := fmt.Sprintf(filepath.Join(dir, "%s.ts"), current.Format("15-04-05"))
	if sg.params.onNewSample != nil {
		sample.Ref()
		sg.params.onNewSample(sample)
		sample.Unref()
	}
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

	_, err = sg.Elements.sink.el.Connect("format-location-full", sg.HandleFormatLocation)
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
			log.Err(err).Msg("unable to link")
			return
		}
		// emit that we get a new source and update the
		// target capabilities
		if sg.params.onNewSource != nil {
			sg.params.onNewSource(pad.GetCurrentCaps())
		}
		// add a probe to get the latest available buffer
		pad.AddProbe(gst.PadProbeTypeBuffer, func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
			if sg.frameBuffer == nil {
				sg.frameBuffer = info.GetBuffer()
				sg.frameBuffer.Ref()
			}
			return gst.PadProbeOK
		})
	}); err != nil {
		return err
	}
	return nil
}

func (sg *SegmentationPipeline) Start(ctx context.Context, pipeline *Pipeline) error {
	ticker := time.NewTicker(sg.params.ensureSegmentDuration)
	// multiple the duration by three to allow processing delay
	wiggleRoom := sg.params.ensureSegmentDuration * 3
	sg.quit = make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				sg.mutex.RLock()
				lastSegmentGenerated := time.Now().Sub(sg.lastFinishedSegment)
				sg.mutex.RUnlock()
				log.Debug().Dur("last-segment-generated-ago", lastSegmentGenerated).Msg("checking last segment generated")
				if lastSegmentGenerated > wiggleRoom {
					// TODO: do we just crash here
					log.Error().Msg("segment has not been processed recently")
				}
			case <-sg.quit:
				log.Debug().Msg("Exiting segment generation check")
				ticker.Stop()
				return
			}
		}
	}()
	return nil
}

func (sg *SegmentationPipeline) Stop(ctx context.Context, pipeline *Pipeline) error {
	close(sg.quit)
	return nil
}
