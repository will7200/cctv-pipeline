package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-gst/go-glib/glib"
	"github.com/rs/zerolog/log"
	"go.uber.org/multierr"
)

type CCTVPipelineParams struct {
	// SourceLocation rtsp uri of where to pull a stream
	SourceLocation string
	// DestinationLocation rtsp uri of where to push a stream
	DestinationLocation string
	// CameraID
	CameraID string
	// SegmentBasePath base path of where to store segments
	SegmentBasePath string
	// TargetVideoSegmentationDuration at what duration to make video segments
	TargetVideoSegmentationDuration time.Duration
}

type InternalError struct {
	error  error
	target *Pipeline
}

func (i InternalError) Error() string {
	return fmt.Sprintf("Error occurred in pipeline %s: %s", i.target.name, i.error)
}

type CCTVPipeline struct {
	stream       *StreamPipeline
	source       *SourcePipeline
	thumbNail    *ThumbnailPipeline
	segmentation *SegmentationPipeline

	basePipeline      *Pipeline
	thumbnailPipeline *Pipeline

	wg     *sync.WaitGroup
	errors chan InternalError
	quit   chan struct{}
}

// NewCCTVPipeline constructs a new pipeline accordingly that cctv supports
func NewCCTVPipeline(params CCTVPipelineParams) (_ *CCTVPipeline, err error) {
	cctv := new(CCTVPipeline)
	cctv.stream, err = NewStreamPipeline(StreamPipelineParams{RtspSink: params.DestinationLocation})
	if err != nil {
		return
	}
	cctv.source, err = NewSourcePipeline(SourcePipelineParams{RtspUri: params.SourceLocation})
	if err != nil {
		return
	}

	cctv.thumbNail, err = NewThumbnailPipeline(ThumbnailParams{
		segmentBasePath: params.SegmentBasePath,
		cameraId:        params.CameraID,
	})
	if err != nil {
		return
	}

	// thumbnail pipeline runs in a separate pipeline
	// since we selectively push buffers on it when a new segment is produced
	thumbnail := NewPipeline("cctv-thumbnail-pipeline")
	thumbnail.AddPartialPipeline(cctv.thumbNail)
	if err = thumbnail.Build(); err != nil {
		return
	}
	cctv.segmentation, err = NewSegmentationPipeline(SegmentPipelineParams{
		videoDuration:         params.TargetVideoSegmentationDuration,
		cameraId:              params.CameraID,
		segmentBasePath:       params.SegmentBasePath,
		ensureSegmentDuration: params.TargetVideoSegmentationDuration,
		onNewSource: func(src *Element) {
			log.Info().Str("source", src.el.GetName()).Msg("Attempting to connect to thumbnail-pipeline")
			err := cctv.thumbNail.Connect(src)
			if err != nil {
				log.Err(err).Msg("Unable to connect src element for thumbnail-pipeline")
			}
		},
		onNewFileSegmentCreate: func(file string) {
			cctv.thumbNail.flush()
		},
	})

	pipeline := NewPipeline("cctv-pipeline")
	pipeline.AddPartialPipeline(cctv.stream)
	pipeline.AddPartialPipeline(cctv.source)
	pipeline.AddPartialPipeline(cctv.segmentation)

	if err = pipeline.Build(); err != nil {
		return
	}

	// connect source -> streaming
	if err = cctv.stream.Connect(cctv.source.Elements.tee); err != nil {
		return
	}

	// connect source -> segmentation
	if err = cctv.segmentation.Connect(cctv.source.Elements.tee); err != nil {
		return
	}

	cctv.wg = &sync.WaitGroup{}
	cctv.basePipeline = pipeline
	cctv.thumbnailPipeline = thumbnail
	cctv.errors = make(chan InternalError, 2)
	return cctv, nil
}

// Start pipeline
func (cctv *CCTVPipeline) Start(ctx context.Context) error {
	runPipelines := []*Pipeline{cctv.basePipeline, cctv.thumbnailPipeline}
	for _, val := range runPipelines {
		cctv.wg.Add(1)
		go func(pipeline *Pipeline) {
			defer cctv.wg.Done()
			pipeline.pipeline.Ref()
			defer pipeline.pipeline.Unref()

			var err error
			complete := make(chan struct{})
			// start loop
			loop := glib.NewMainLoop(glib.MainContextDefault(), false)
			go func() {
				err = pipeline.Start(ctx, loop)
				pipeline.Finish(ctx)
				complete <- struct{}{}
			}()

			select {
			case <-ctx.Done():
				break
			case <-complete:
				break
			}
			if err != nil {
				cctv.errors <- InternalError{
					error:  err,
					target: pipeline,
				}
			}
			loop.Quit()
		}(val)
	}
	return nil
}

// Run until an error occurs
func (cctv *CCTVPipeline) Run() error {
	for err := range cctv.errors {
		return err
	}
	return nil
}

// Wait for all pipelines to stop running
func (cctv *CCTVPipeline) Wait() {
	cctv.wg.Wait()
}

// Stop all pipelines
func (cctv *CCTVPipeline) Stop() error {
	runPipelines := []*Pipeline{cctv.basePipeline, cctv.thumbnailPipeline}
	var errors []error
	for _, val := range runPipelines {
		if val.loop == nil {
			errors = append(errors, fmt.Errorf("%s pipeline was never started", val.name))
			continue
		}
		val.loop.Quit()
	}
	if len(errors) > 0 {
		return multierr.Combine(errors...)
	}
	cctv.Wait()
	close(cctv.errors)
	for err := range cctv.errors {
		errors = append(errors, err)
	}
	return multierr.Combine(errors...)
}
