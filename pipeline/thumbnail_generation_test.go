package pipeline

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
	"github.com/stretchr/testify/assert"
)

func TestThumbnailPipelineFromFile(t *testing.T) {
	_, err := os.Stat(TestH265FileName)
	assert.Nil(t, err)
	// setup pipeline
	source := NewFileSrcElement("source", TestH265FileName)
	decoded := NewDecodeElement("bin")

	segmentBase := newSegmentBase()

	tpipeline, err := NewThumbnailPipeline(ThumbnailParams{
		segmentBasePath: segmentBase,
		cameraId:        TestCameraID,
	})
	assert.Nil(t, err)
	spipline, err := NewSegmentationPipeline(SegmentPipelineParams{
		videoDuration:         time.Second,
		cameraId:              TestCameraID,
		segmentBasePath:       segmentBase,
		ensureSegmentDuration: time.Second,
		onNewSource: func(element *Element) {
			assert.Nil(t, tpipeline.Connect(element))
		},
		onNewFileSegmentCreate: func(file string) {
			tpipeline.flush()
		},
	})
	assert.Nil(t, err)

	pipeline := NewPipeline("segment-pipeline")
	pipeline.AddPartialPipeline(spipline)
	//pipeline.AddPartialPipeline(tpipeline)
	pipeline.AddElements(source, decoded)
	assert.Nil(t, pipeline.Build())
	assert.Nil(t, source.Link(decoded))
	assert.Nil(t, spipline.Connect(decoded))
	pipeline.pipeline.DebugBinToDotFileWithTs(gst.DebugGraphShowAll, "thumbNail")
	thumbnail := NewPipeline("thumbnail-pipeline")
	thumbnail.AddPartialPipeline(tpipeline)
	assert.Nil(t, thumbnail.Build())

	// setup context
	waitFor := time.Second * 12
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, waitFor)
	time.AfterFunc(waitFor, cancel)

	go func() {
		assert.Nil(t, runPipeline(ctx, thumbnail))
	}()
	assert.Nil(t, runPipeline(ctx, pipeline))
	cancel()
	thumbnail.pipeline.DebugBinToDotFileWithTs(gst.DebugGraphShowAll, "thumbNail")
	assert.GreaterOrEqual(t, tpipeline.state.createCount, uint(8))
}

func runPipeline(ctx context.Context, pipeline *Pipeline) error {
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
	loop.Quit()
	return err
}
