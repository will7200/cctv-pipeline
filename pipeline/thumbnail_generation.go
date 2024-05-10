package pipeline

import (
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

type ThumbnailPipelineElements struct {
	appsrc     *Element
	webpenc    *Element
	sink       *Element
	src        *app.Source
	converter  *Element
	scaler     *Element
	customSink *app.Sink
}

type ThumbnailParams struct {
	// segmentBasePath base file path
	segmentBasePath string
	// cameraId this camera
	cameraId string
}

// ThumbnailPipeline will split a given stream into smaller parts
type ThumbnailPipeline struct {
	// elements
	Elements ThumbnailPipelineElements
	// parameters
	params ThumbnailParams
	// source element
	source *Element
	// samples
	samples []*gst.Sample
	mutex   sync.RWMutex
}

// NewThumbnailPipeline create a new segmentation pipeline
func NewThumbnailPipeline(params ThumbnailParams) (sg *ThumbnailPipeline, err error) {
	sg = new(ThumbnailPipeline)
	sg.params = params
	sg.mutex = sync.RWMutex{}

	sg.Elements.converter = &Element{
		Factory: "videoconvert",
		Name:    "converter",
	}

	sg.Elements.appsrc = &Element{
		Factory: "appsrc",
		Name:    "appsrc",
		Properties: map[string]interface{}{
			"is-live":    true,
			"leaky-type": 1,
		},
	}

	sg.Elements.scaler = &Element{
		Factory: "videoscale",
		Name:    "videoscale",
	}

	sg.Elements.webpenc = &Element{
		Factory: "webpenc",
		Name:    "webpenc",
		Properties: map[string]interface{}{
			"quality": float32(.1),
		},
	}

	sg.Elements.sink = &Element{
		Factory:    "appsink",
		Name:       "customSink",
		Properties: map[string]interface{}{},
	}

	return
}

func (th *ThumbnailPipeline) Prepare(pipeline *Pipeline) error {
	spElements := []*Element{
		th.Elements.appsrc,
		th.Elements.webpenc,
		th.Elements.sink,
		th.Elements.converter,
		th.Elements.scaler,
	}
	pipeline.AddElements(spElements...)
	return nil
}

func (th *ThumbnailPipeline) Build(pipeline *Pipeline) error {
	links := []LinkWithCaps{
		{th.Elements.appsrc, th.Elements.converter, nil},
		{th.Elements.converter, th.Elements.scaler, nil},
		{th.Elements.scaler, th.Elements.webpenc, gst.NewCapsFromString("video/x-raw,width=1280,height=720")},
		{th.Elements.webpenc, th.Elements.sink, nil},
	}
	for _, link := range links {
		if link.filter == nil {
			if err := link.left.Link(link.right); err != nil {
				return err
			}
		} else {
			if err := link.left.LinkFiltered(link.right, link.filter); err != nil {
				return err
			}
		}
	}

	src := app.SrcFromElement(th.Elements.appsrc.el)
	th.Elements.src = src

	th.Elements.customSink = app.SinkFromElement(th.Elements.sink.el)

	index := 0
	// Getting data out of the appsink is done by setting callbacks on it.
	// The appsink will then call those handlers, as soon as data is available.
	th.Elements.customSink.SetCallbacks(&app.SinkCallbacks{
		// Add a "new-sample" callback
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
			//
			// We also know what format to expect because we set it with the caps. So we convert
			// the map directly to signed 16-bit little-endian integers.
			samples := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()
			dir := filepath.Join(th.params.segmentBasePath, th.params.cameraId)
			err := os.MkdirAll(dir, os.FileMode(0744))
			if err != nil {
				log.Err(err).Msg("unable to create directory")
			}
			err = os.WriteFile(fmt.Sprintf(filepath.Join(th.params.segmentBasePath, th.params.cameraId, "%d_thumb.webp"), index), samples, 0744)
			if err != nil {
				log.Err(err).Msg("Unable to write to file")
			}
			index++
			return gst.FlowOK
		},
	})

	return nil
}

func (th *ThumbnailPipeline) CopySample(sample *gst.Sample) {
	ret := th.Elements.src.PushSample(sample.Copy())
	if ret != gst.FlowOK {
		log.Err(errors.New("Flow return is not ok")).Msg(ret.String())
	}
}

func (th *ThumbnailPipeline) OnBuffer(buffer *gst.Buffer) {
	th.Elements.src.PushBuffer(buffer)
}

func (th *ThumbnailPipeline) Connect(source *Element) error {
	th.source = source
	return nil
}

func (th *ThumbnailPipeline) Start(pipeline *Pipeline) error {
	return nil
}

func (th *ThumbnailPipeline) Stop(pipeline *Pipeline) error {
	return nil
}
