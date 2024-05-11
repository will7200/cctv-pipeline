package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/go-gst/go-glib/glib"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gopkg.in/vansante/go-ffprobe.v2"
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
		segmentBasePath:       fmt.Sprintf("./tmp/%s", uuid.New().String()[:10]),
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
		err = pipeline.Start(ctx, loop)
		assert.Nil(t, err)
		cancel()
		pipeline.Finish(ctx)
	}()

	select {
	case <-ctx.Done():
		break
	}
	loop.Quit()

	assert.Equal(t, spipline.SegmentCounter, uint32(10))

	entries, err := filepath.Glob(fmt.Sprintf("%s/*/*/*/*/*.ts", spipline.params.segmentBasePath))
	assert.Nil(t, err)
	for _, val := range entries {
		ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelFn()
		data, err := ffprobe.ProbeURL(ctx, val)
		assert.Nil(t, err)
		duration, err := strconv.ParseFloat(data.Streams[0].Duration, 64)
		assert.Nil(t, err)
		assert.InDeltaf(t, duration, 1, .1, "Expected duration to be around 1")
	}
}
