package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/rs/zerolog/log"
)

var _ PartialPipeline = (*ThumbnailPipeline)(nil)

type ThumbnailPipelineState struct {
	createCount uint
	// holds the first frame available for the current segment
	frameBuffer  *gst.Buffer
	currentProbe uint64
}

type ThumbnailPipelineElements struct {
	appsrc     *Element
	webpenc    *Element
	sink       *Element
	src        *app.Source
	converter  *Element
	scaler     *Element
	customSink *app.Sink
	capsfilter *Element
}

type ThumbnailParams struct {
	// segmentBasePath base file path
	segmentBasePath string
	// cameraId this camera
	cameraId string
	// width of image, defaults to 1280 if zero
	width uint
	// height of image, defaults to 720 if zero
	height uint
}

// Caps generates *gst.Caps for the provided image dimension
func (tp ThumbnailParams) Caps() *gst.Caps {
	return gst.NewCapsFromString(fmt.Sprintf("video/x-raw,width=%d,height=%d", tp.width, tp.height))
}

// ThumbnailPipeline will split a given stream into smaller parts
type ThumbnailPipeline struct {
	// elements
	Elements ThumbnailPipelineElements
	// parameters
	params ThumbnailParams
	// source element
	source *Element
	// state
	state    *ThumbnailPipelineState
	mutex    sync.RWMutex
	pipeline *Pipeline
}

// NewThumbnailPipeline create a new thumbnail generation pipeline
func NewThumbnailPipeline(params ThumbnailParams) (sg *ThumbnailPipeline, err error) {
	sg = new(ThumbnailPipeline)
	sg.state = new(ThumbnailPipelineState)
	sg.params = params
	sg.mutex = sync.RWMutex{}

	if sg.params.height == 0 {
		sg.params.height = 720
	}
	if sg.params.width == 0 {
		sg.params.width = 1280
	}

	return
}

func (th *ThumbnailPipeline) NewElements() error {
	th.Elements.converter = &Element{
		Factory: "videoconvert",
		Name:    "converter",
	}

	th.Elements.appsrc = &Element{
		Factory: "appsrc",
		Name:    "appsrc",
		Properties: map[string]interface{}{
			"is-live": true,
			//"leaky-type": 1,
		},
	}

	th.Elements.scaler = &Element{
		Factory: "videoscale",
		Name:    "videoscale",
	}

	th.Elements.capsfilter = &Element{
		Factory: "capsfilter",
		Name:    "capsfilter",
	}

	th.Elements.webpenc = &Element{
		Factory: "webpenc",
		Name:    "webpenc",
		Properties: map[string]interface{}{
			"quality": float32(.1),
		},
	}

	th.Elements.sink = &Element{
		Factory:    "appsink",
		Name:       "customSink",
		Properties: map[string]interface{}{},
	}

	elements := th.allElements()
	for _, element := range elements {
		err := element.Build()
		if err != nil {
			return err
		}
	}

	src := app.SrcFromElement(th.Elements.appsrc.el)

	th.Elements.src = src

	th.Elements.customSink = app.SinkFromElement(th.Elements.sink.el)
	th.Elements.customSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			// Pull the sample that triggered this callback
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}

			// Retrieve the buffer from the sample
			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}

			// At this point, buffer is only a reference to an existing memory region somewhere.
			// When we want to access its content, we have to map it while requesting the required
			// mode of access (read, read/write).
			samples := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()
			dir := filepath.Join(th.params.segmentBasePath, th.params.cameraId)
			err := os.MkdirAll(dir, os.FileMode(0744))
			if err != nil {
				log.Err(err).Msg("unable to create directory")
			}
			// Update create count
			th.mutex.Lock()
			th.state.createCount++
			th.mutex.Unlock()

			file := fmt.Sprintf(filepath.Join(th.params.segmentBasePath, th.params.cameraId, "%d_thumb.webp"), th.state.createCount)
			err = os.WriteFile(file, samples, 0744)
			if err != nil {
				th.mutex.Lock()
				th.state.createCount--
				th.mutex.Unlock()
				log.Err(err).Msg("Unable to write to file")
				return gst.FlowOK
			}
			log.Info().Str("file", file).Msg("Created new thumbnail")
			return gst.FlowOK
		},
	})
	return nil
}

func (th *ThumbnailPipeline) allElements() []*Element {
	spElements := []*Element{
		th.Elements.appsrc,
		th.Elements.webpenc,
		th.Elements.capsfilter,
		th.Elements.sink,
		th.Elements.converter,
		th.Elements.scaler,
	}
	return spElements
}

func (th *ThumbnailPipeline) allGSTElements() []*gst.Element {
	spElements := []*gst.Element{
		th.Elements.appsrc.el,
		th.Elements.webpenc.el,
		th.Elements.capsfilter.el,
		th.Elements.sink.el,
		th.Elements.converter.el,
		th.Elements.scaler.el,
	}
	return spElements
}

func (th *ThumbnailPipeline) links() []LinkWithCaps {
	links := []LinkWithCaps{
		{th.Elements.appsrc, th.Elements.converter, nil},
		{th.Elements.converter, th.Elements.scaler, nil},
		{th.Elements.scaler, th.Elements.capsfilter, nil},
		{th.Elements.capsfilter, th.Elements.webpenc, nil},
		{th.Elements.webpenc, th.Elements.sink, nil},
	}
	return links
}

func (th *ThumbnailPipeline) Prepare(pipeline *Pipeline) error {
	err := th.NewElements()
	pipeline.AddElements(th.allElements()...)
	return err
}

func (th *ThumbnailPipeline) Build(pipeline *Pipeline) error {
	th.pipeline = pipeline
	return nil
}

// CopySample will push a *gst.Sample to appsrc, pushing a sample
// has the benefits that the *gst.Caps get set onto the appsrc
func (th *ThumbnailPipeline) CopySample(sample *gst.Sample) {
	ret := th.Elements.src.PushSample(sample.Copy())
	if ret != gst.FlowOK {
		log.Err(errors.New("Flow return is not ok")).Msg(ret.String())
	}
}

// OnBuffer connects pushes a buffer onto the appsrc buffer to be processed
// it is expected that the appsink's *gst.Caps get updated before calling this method
func (th *ThumbnailPipeline) OnBuffer(buffer *gst.Buffer) {
	ret := th.Elements.src.PushBuffer(buffer)
	if ret != gst.FlowOK {
		log.Err(errors.New("Flow return is not ok")).Msg(ret.String())
	}
}

func (th *ThumbnailPipeline) Connect(source *Element) error {
	th.mutex.Lock()
	defer th.mutex.Unlock()
	th.source = source
	pad := th.source.el.GetStaticPad("src")
	caps := pad.GetCurrentCaps()
	if caps == nil {
		return errors.New("source does not have caps")
	}
	unlink := false
	if th.state.currentProbe != 0 {
		unlink = true
		pad.RemoveProbe(th.state.currentProbe)
	}
	th.state.currentProbe = pad.AddProbe(gst.PadProbeTypeBuffer, func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		th.mutex.Lock()
		defer th.mutex.Unlock()

		if th.state.frameBuffer == nil {
			if frameBuffer := info.GetBuffer(); frameBuffer != nil {
				th.state.frameBuffer = frameBuffer
				th.state.frameBuffer.Ref()
			}
		}
		return gst.PadProbeOK
	})
	var err error
	err = th.pipeline.pipeline.SetState(gst.StateNull)
	if err != nil {
		return err
	}
	err = th.Elements.src.SetState(gst.StateNull)
	if err != nil {
		return err
	}
	if unlink {
		links := th.links()
		for _, link := range links {
			err := link.left.el.SetState(gst.StateNull)
			if err != nil {
				return err
			}
			err = link.right.el.SetState(gst.StateNull)
			if err != nil {
				return err
			}
			err = link.left.Unlink(link.right)
			if err != nil {
				return err
			}
		}
		for _, ele := range th.allElements() {
			err := th.pipeline.pipeline.Remove(ele.el)
			if err != nil {
				return err
			}
		}

		err = th.NewElements()
		if err != nil {
			return err
		}
		err = th.pipeline.pipeline.AddMany(th.allGSTElements()...)
		if err != nil {
			return err
		}
	}
	err = th.Elements.capsfilter.el.SetProperty("caps", th.params.Caps())
	if err != nil {
		return err
	}
	th.Elements.src.SetCaps(caps)
	err = th.Elements.appsrc.el.SetState(gst.StatePlaying)
	if err != nil {
		return err
	}
	links := th.links()
	for _, link := range links {
		link.left.el.SyncStateWithParent()
		if err := link.left.LinkFiltered(link.right, link.filter); err != nil {
			return err
		}
	}
	th.Elements.sink.el.SyncStateWithParent()
	if th.state.frameBuffer != nil {
		th.state.frameBuffer.Unref()
	}
	th.state.frameBuffer = nil
	if err != nil {
		return err
	}
	err = th.pipeline.pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return err
	}
	return nil
}

func (th *ThumbnailPipeline) Start(ctx context.Context, pipeline *Pipeline) error {
	return nil
}

func (th *ThumbnailPipeline) Stop(ctx context.Context, pipeline *Pipeline) error {
	return nil
}

func (th *ThumbnailPipeline) flush() {
	th.mutex.Lock()
	defer th.mutex.Unlock()
	if th.state.frameBuffer == nil {
		log.Error().Msg("No buffer")
		return
	}
	ret := th.Elements.src.PushBuffer(th.state.frameBuffer)
	if ret != gst.FlowOK {
		log.Err(errors.New("Flow return is not ok")).Str("state", th.Elements.appsrc.el.GetCurrentState().String()).Msg(ret.String())
	}
	th.state.frameBuffer.Unref()
	th.state.frameBuffer = nil
}
