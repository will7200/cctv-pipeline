package pipeline

import (
	"context"
	"sync"
)

var _ PartialPipeline = (*StreamPipeline)(nil)

type StreamPipelineElements struct {
}

type StreamPipelineParams struct {
}

// StreamPipeline will split a given stream into smaller parts
type StreamPipeline struct {
	// elements
	Elements StreamPipelineElements
	// parameters
	params SegmentPipelineParams
	// source element
	source *Element
	// quit holds a chan to send to stop checking this part of the pipeline
	quit  chan struct{}
	mutex sync.RWMutex
}

func (s *StreamPipeline) Prepare(pipeline *Pipeline) error {
	//TODO implement me
	panic("implement me")
}

func (s *StreamPipeline) Build(pipeline *Pipeline) error {
	//TODO implement me
	panic("implement me")
}

func (s *StreamPipeline) Connect(source *Element) error {
	//TODO implement me
	panic("implement me")
}

func (s *StreamPipeline) Start(ctx context.Context, pipeline *Pipeline) error {
	//TODO implement me
	panic("implement me")
}

func (s *StreamPipeline) Stop(ctx context.Context, pipeline *Pipeline) error {
	//TODO implement me
	panic("implement me")
}
