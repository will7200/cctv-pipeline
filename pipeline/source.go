package pipeline

import (
	"context"
	"sync"

	"github.com/go-gst/go-gst/gst"
	"github.com/rs/zerolog/log"
)

var _ PartialPipeline = (*SourcePipeline)(nil)

type SourcePipelineElements struct {
	rtspsrc *Element
	depay   *Element
	tee     *Element
}

type SourcePipelineParams struct {
	RtspUri string
}

// SourcePipeline will split a given Source into smaller parts
type SourcePipeline struct {
	// elements
	Elements SourcePipelineElements
	// parameters
	params SourcePipelineParams
	// quit holds a chan to send to stop checking this part of the pipeline
	quit  chan struct{}
	mutex sync.RWMutex
}

// SourcePipeline create a new source pipeline
func NewSourcePipeline(params SourcePipelineParams) (sp *SourcePipeline, err error) {
	sp = new(SourcePipeline)
	sp.params = params
	sp.mutex = sync.RWMutex{}

	sp.Elements.rtspsrc = &Element{
		Factory: "rtspsrc",
		Name:    "src",
		Properties: map[string]interface{}{
			"location": sp.params.RtspUri,
		},
	}

	sp.Elements.depay = &Element{
		Factory: "unknown",
		Name:    "depay",
	}

	sp.Elements.tee = &Element{
		Factory: "tee",
		Name:    "tee",
	}

	return
}

func (s *SourcePipeline) Prepare(pipeline *Pipeline) error {
	spElements := []*Element{
		s.Elements.rtspsrc,
		s.Elements.tee,
	}
	pipeline.AddElements(spElements...)
	return nil
}

func (s *SourcePipeline) Build(pipeline *Pipeline) error {
	if _, err := s.Elements.rtspsrc.el.Connect("pad-added", func(src *gst.Element, pad *gst.Pad) {
		log.Printf("Pad '%s' has caps %s", pad.GetName(), pad.GetCurrentCaps().String())
		if pad.IsLinked() {
			log.Printf("Pad '%s' is already linked", pad.GetName())
			return
		}
		caps := pad.GetCurrentCaps()
		encodingName, err := caps.GetStructureAt(0).GetValue("encoding-name")
		if err != nil {
			log.Print("can't get encoding")
			return
		}
		var desiredCaps *gst.Caps
		switch encodingName {
		case "H264":
			s.Elements.depay.Factory = "rtph264depay"
			desiredCaps = gst.NewCapsFromString("video/x-h264, stream-format=byte-stream, alignment=au")
		case "H265":
			s.Elements.depay.Factory = "rtph265depay"
			desiredCaps = gst.NewCapsFromString("video/x-h265, stream-format=byte-stream, alignment=au")
		default:
			log.Printf("Unsupported encoding: %s", encodingName)
		}
		elements := []*Element{
			s.Elements.depay,
		}
		for _, elem := range elements {
			if err := elem.Build(); err != nil {
				log.Err(err).Msg("can't build")
				return
			}
			if err := pipeline.pipeline.Add(elem.el); err != nil {
				log.Err(err).Msg("can't add element")
				return
			}
		}
		if desiredCaps == nil {
			log.Panic().Msg("desired caps is nil")
		}
		err = src.Link(s.Elements.depay.el)
		if err != nil {
			log.Err(err).Msg("can't link depay")
			return
		}
		err = s.Elements.depay.LinkFiltered(s.Elements.tee, desiredCaps)
		if err != nil {
			log.Err(err).Msg("can't link tee")
			return
		}
		s.Elements.depay.el.SyncStateWithParent()
	}); err != nil {
		return err
	}
	return nil
}

func (s *SourcePipeline) Connect(source *Element) error {
	//TODO implement me
	return nil
}

func (s *SourcePipeline) Start(ctx context.Context, pipeline *Pipeline) error {
	//TODO implement me
	return nil
}

func (s *SourcePipeline) Stop(ctx context.Context, pipeline *Pipeline) error {
	//TODO implement me
	return nil
}
