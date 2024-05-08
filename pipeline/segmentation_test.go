package pipeline

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/go-gst/go-glib/glib"
)

func TestSegmentPipelineFromFile(t *testing.T) {
	_, err := os.Stat(TestH265FileName)
	assert.Nil(t, err)
	// setup pipeline
	source := NewFileSrcElement("source", TestH265FileName)
	decoded := NewDecodeElement("bin")

	spipline, err := NewSegmentationPipeline(SegmentPipelineParams{
		videoDuration:         time.Second,
		cameraId:              "test",
		segmentBasePath:       "./tmp",
		ensureSegmentDuration: time.Second,
	})
	assert.Nil(t, err)

	pipeline := NewPipeline("segment-pipeline")
	pipeline.AddPartialPipeline(spipline)
	pipeline.AddElements(source, decoded)
	assert.Nil(t, pipeline.Build())
	assert.Nil(t, source.Link(decoded))
	assert.Nil(t, spipline.Connect(decoded))

	// setup context
	waitFor := time.Second * 30
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, waitFor)
	time.AfterFunc(waitFor, cancel)

	// start loop
	loop := glib.NewMainLoop(glib.MainContextDefault(), false)
	go func() {
		err = pipeline.Start(loop)
		assert.Nil(t, err)
		cancel()
		pipeline.Finish()
	}()

	select {
	case <-ctx.Done():
		break
	}
	loop.Quit()

	assert.Equal(t, spipline.SegmentCounter, uint32(10))
}
