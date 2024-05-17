package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
	"github.com/rs/zerolog/log"
)

var _ PartialPipeline = (*StreamPipeline)(nil)

type StreamPipelineState struct {
	encodingName string
}

type StreamPipelineElements struct {
	bin         *gst.Bin
	queue       *Element
	streamInput *gst.GhostPad
	sink        *Element
	decoder     *Element
	converter   *Element
	encoder     *Element
	mux         *Element
}

type StreamPipelineParams struct {
	// RtspSink location of where to send stream
	RtspSink string
}

// StreamPipeline will split a given stream into smaller parts
type StreamPipeline struct {
	// elements
	Elements StreamPipelineElements
	// parameters
	params StreamPipelineParams
	// source element
	source *Element
	// state
	state *StreamPipelineState
	// quit holds a chan to send to stop checking sis part of se pipeline
	quit  chan struct{}
	mutex sync.RWMutex
}

func NewStreamPipeline(params StreamPipelineParams) (s *StreamPipeline, err error) {
	s = new(StreamPipeline)
	s.params = params
	s.state = new(StreamPipelineState)
	s.Elements = StreamPipelineElements{
		queue: &Element{
			Factory: "queue",
			Name:    "stream-queue",
			Properties: map[string]interface{}{
				"leaky": 2,
			},
		},
		sink: &Element{
			Factory: "rtspclientsink",
			Name:    "destination",
			Properties: map[string]interface{}{
				"location":  params.RtspSink,
				"protocols": uint(4), // tcp
			},
			el: nil,
		},
		decoder: nil,
		encoder: &Element{
			Factory: "x264enc",
			Name:    "encoder",
			Properties: map[string]interface{}{
				"speed-preset": 1,
				"tune":         uint(4),
				"key-int-max":  uint(10),
			},
		},
		converter: &Element{
			Factory: "videoconvert",
			Name:    "converter",
		},
		mux: &Element{
			Factory:    "mpegtsmux",
			Name:       "muxer",
			Properties: nil,
			el:         nil,
		},
	}
	return
}

func (s *StreamPipeline) Prepare(pipeline *Pipeline) error {
	return nil
}

func (s *StreamPipeline) HandleStreamChange(newState StreamPipelineState) (err error) {
	if s.state.encodingName != "" {
		// TODO: handle stream changes
		return errors.New("stream change detected, unimplemented")
	}
	switch newState.encodingName {
	case "video/x-h264":
		elements := []*Element{}
		for _, elem := range elements {
			if err := elem.Build(); err != nil {
				return err
			}
			if err := s.Elements.bin.Add(elem.el); err != nil {
				return err
			}
		}
		// link straight to converter
		links := []LinkWithCaps{
			{s.Elements.queue, s.Elements.mux, nil},
		}
		for _, link := range links {
			if err := link.left.LinkFiltered(link.right, link.filter); err != nil {
				return err
			}
		}
		s.Elements.mux.el.SyncStateWithParent()
	case "video/x-h265":
		if s.Elements.decoder != nil {
			// TODO: remove links
			err := s.Elements.bin.Remove(s.Elements.decoder.el)
			if err != nil {
				return err
			}
		}
		s.Elements.decoder = &Element{
			Factory: "avdec_h265",
			Name:    "decoder",
		}
		elements := []*Element{
			s.Elements.decoder,
			s.Elements.encoder,
			s.Elements.converter,
		}
		for _, elem := range elements {
			if err := elem.Build(); err != nil {
				return err
			}
			if err := s.Elements.bin.Add(elem.el); err != nil {
				return err
			}
		}
		links := []LinkWithCaps{
			{s.Elements.queue, s.Elements.decoder, nil},
			{s.Elements.decoder, s.Elements.converter, nil},
			{s.Elements.converter, s.Elements.encoder, nil},
			{s.Elements.encoder, s.Elements.mux, nil},
		}
		for _, link := range links {
			if err := link.left.LinkFiltered(link.right, link.filter); err != nil {
				return err
			}
		}
		s.Elements.decoder.el.SyncStateWithParent()
	default:
		return errors.New(fmt.Sprintf("Encoding not supported %s", newState.encodingName))
	}
	s.state = &newState
	return nil
}

func (s *StreamPipeline) Build(pipeline *Pipeline) error {
	s.Elements.bin = gst.NewBin("stream-bin")
	elements := []*Element{
		s.Elements.queue,
		s.Elements.sink,
		s.Elements.mux,
	}

	if err := pipeline.pipeline.Add(s.Elements.bin.Element); err != nil {
		return err
	}
	for _, elem := range elements {
		if err := elem.Build(); err != nil {
			return err
		}
		if err := s.Elements.bin.Add(elem.el); err != nil {
			return err
		}
	}
	pad := s.Elements.queue.el.GetStaticPad("src")
	_, err := pad.Connect("notify::caps", func(pad *gst.Pad, parameter *glib.ParamSpec) {
		s.mutex.Lock()
		defer s.mutex.Unlock()
		caps := pad.GetCurrentCaps()
		if caps == nil {
			return
		}
		encodingName := caps.GetStructureAt(0).Name()
		if encodingName == s.state.encodingName {
			return
		}
		log.Info().Str("old", s.state.encodingName).Str("new", encodingName).Msg("Detected stream change")
		err := pipeline.pipeline.SetState(gst.StatePaused)
		if err != nil {
			log.Err(err).Msg("Unable to pause pipeline")
		}
		err = s.HandleStreamChange(StreamPipelineState{encodingName: encodingName})
		if err != nil {
			log.Err(err).Msg("Unable to handle stream change")
			pipeline.Quit()
		}
		err = pipeline.pipeline.SetState(gst.StatePlaying)
		if err != nil {
			log.Err(err).Msg("Unable to start pipeline")
		}
	})
	if err != nil {
		return err
	}
	links := []LinkWithCaps{
		{s.Elements.mux, s.Elements.sink, nil},
	}
	for _, link := range links {
		if err := link.left.LinkFiltered(link.right, link.filter); err != nil {
			return err
		}
	}

	return nil
}

func (s *StreamPipeline) Connect(source *Element) error {
	s.source = source
	err := source.Link(s.Elements.queue)
	if err != nil {
		return err
	}
	return nil
}

func (s *StreamPipeline) Start(ctx context.Context, pipeline *Pipeline) error {
	return nil
}

func (s *StreamPipeline) Stop(ctx context.Context, pipeline *Pipeline) error {
	return nil
}
