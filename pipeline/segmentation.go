package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
	"github.com/rs/zerolog/log"
)

var _ PartialPipeline = (*SegmentationPipeline)(nil)

type SegmentationPipelineState struct {
	encodingName string
}

type SegmentationPipelineElements struct {
	converter *Element
	encoder   *Element
	parser    *Element
	sink      *Element
	queue     *Element
	bin       *gst.Bin
	decoder   *Element
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
	// callback for new source
	onNewSource func(caps *gst.Caps)
	// callback for when new segment is created
	onNewFileSegmentCreate func(file string)
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
	// state
	state *SegmentationPipelineState
	// lastFinishedSegment is the time of the last finished segment
	lastFinishedSegment time.Time
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
	sg.state = new(SegmentationPipelineState)

	sg.Elements = SegmentationPipelineElements{
		queue: &Element{
			Factory: "queue",
			Name:    "segmentation-input-queue",
			Properties: map[string]interface{}{
				"max-size-bytes": uint(1024 * 1024 * 10),
			},
		},
		decoder: nil,
	}

	sg.Elements.converter = &Element{
		Factory: "videoconvert",
		Name:    "converter",
	}

	sg.Elements.encoder = &Element{
		Factory: "encoder",
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
				if sg.params.onNewFileSegmentCreate != nil {
					sg.params.onNewFileSegmentCreate(val.(string))
				}
				return true
			} else {
				sg.fileSegmentTracker = false
			}
			runTime, _ := structure.GetValue("running-time")
			gRunTime := runTime.(uint64)
			log.Printf("New file created %s with current run-time of %v", val, gRunTime)
			sg.mutex.Lock()
			sg.lastFinishedSegment = time.Now()
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

func (sg *SegmentationPipeline) HandleStreamChange(newState SegmentationPipelineState) (err error) {
	if sg.state.encodingName != "" {
		// TODO: handle stream changes
		return errors.New("stream change detected, unimplemented")
	}
	switch newState.encodingName {
	case "video/x-raw":
		pad := sg.Elements.queue.el.GetStaticPad("src")
		// emit that we get a new source and update the
		// target capabilities
		if sg.params.onNewSource != nil {
			sg.params.onNewSource(pad.GetCurrentCaps())
		}
		sg.Elements.encoder.Factory = "x265enc"
		elements := []*Element{
			sg.Elements.encoder,
			sg.Elements.converter,
			sg.Elements.parser,
		}
		for _, elem := range elements {
			if err := elem.Build(); err != nil {
				return err
			}
			if err := sg.Elements.bin.Add(elem.el); err != nil {
				return err
			}
		}
		links := []LinkWithCaps{
			{sg.Elements.queue, sg.Elements.converter, nil},
			{sg.Elements.converter, sg.Elements.encoder, nil},
			{sg.Elements.encoder, sg.Elements.parser, nil},
			{sg.Elements.parser, sg.Elements.sink, nil},
		}
		for _, link := range links {
			link.left.el.SyncStateWithParent()
			if err := link.left.LinkFiltered(link.right, link.filter); err != nil {
				return err
			}
		}
	//case "video/x-h264":
	//	return errors.New("supp")
	case "video/x-h265":
		sg.Elements.decoder = &Element{
			Factory: "avdec_h265",
			Name:    "decoder",
		}
		elements := []*Element{
			sg.Elements.decoder,
			sg.Elements.encoder,
			sg.Elements.converter,
			sg.Elements.parser,
		}
		for _, elem := range elements {
			if err := elem.Build(); err != nil {
				return err
			}
			if err := sg.Elements.bin.Add(elem.el); err != nil {
				return err
			}
		}
		links := []LinkWithCaps{
			{sg.Elements.queue, sg.Elements.decoder, nil},
			{sg.Elements.decoder, sg.Elements.converter, nil},
			{sg.Elements.converter, sg.Elements.encoder, nil},
			{sg.Elements.encoder, sg.Elements.parser, nil},
		}
		for _, link := range links {
			if err := link.left.LinkFiltered(link.right, link.filter); err != nil {
				return err
			}
		}
		sg.Elements.decoder.el.SyncStateWithParent()
	default:
		return errors.New(fmt.Sprintf("Encoding not supported %s", newState.encodingName))
	}
	sg.state = &newState
	return nil
}

func (sg *SegmentationPipeline) Build(pipeline *Pipeline) error {
	var err error
	sg.Elements.bin = gst.NewBin("segmentation-bin")
	elements := []*Element{
		sg.Elements.queue,
		sg.Elements.sink,
	}

	if err = pipeline.pipeline.Add(sg.Elements.bin.Element); err != nil {
		return err
	}
	for _, elem := range elements {
		if err = elem.Build(); err != nil {
			return err
		}
		if err := sg.Elements.bin.Add(elem.el); err != nil {
			return err
		}
	}

	_, err = sg.Elements.sink.el.Connect("format-location-full", sg.HandleFormatLocation)
	if err != nil {
		return err
	}

	pad := sg.Elements.queue.el.GetStaticPad("src")
	_, err = pad.Connect("notify::caps", func(pad *gst.Pad, parameter *glib.ParamSpec) {
		sg.mutex.Lock()
		defer sg.mutex.Unlock()
		caps := pad.GetCurrentCaps()
		var encodingName string
		if caps == nil {
			encodingName = ""
		} else {
			encodingName = caps.GetStructureAt(0).Name()
		}
		if encodingName == sg.state.encodingName {
			return
		}
		log.Info().Str("old", sg.state.encodingName).Str("new", encodingName).Msg("Detected stream change")
		if pipeline.pipeline.GetCurrentState() == gst.StatePaused && encodingName == "" {
			// a stream finished for some reason
			return
		}
		err := pipeline.pipeline.SetState(gst.StatePaused)
		if err != nil {
			log.Err(err).Msg("Unable to pause pipeline")
		}
		err = sg.HandleStreamChange(SegmentationPipelineState{encodingName: encodingName})
		if err != nil {
			log.Err(err).Msg("Unable to handle stream change")
			pipeline.Quit()
		}
		err = pipeline.pipeline.SetState(gst.StatePlaying)
		if err != nil {
			log.Err(err).Msg("Unable to start pipeline")
		}
	})

	return nil
}

func (sg *SegmentationPipeline) Connect(source *Element) error {
	sg.source = source
	pads, err := source.el.GetSrcPads()
	if err != nil {
		return err
	}
	if len(pads) == 1 {
		err := source.Link(sg.Elements.queue)
		if err != nil {
			return err
		}
		return nil
	}

	_, err = sg.source.el.Connect("pad-added", func(src *gst.Element, pad *gst.Pad) {
		log.Printf("Pad '%s' has caps %s", pad.GetName(), pad.GetCurrentCaps().String())
		if pad.IsLinked() {
			log.Printf("Pad '%s' is already linked", pad.GetName())
			return
		}
		mediaType := pad.GetCurrentCaps().GetStructureAt(0).Name()
		if !strings.Contains(mediaType, "video/") {
			return
		}
		err := source.Link(sg.Elements.queue)
		if err != nil {
			log.Err(err).Msg("unable to link element to src")
		}
	})
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
