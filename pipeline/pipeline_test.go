package pipeline

import (
	"context"
	"os"
	"path"
	"testing"
	"time"

	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"

	"github.com/stretchr/testify/assert"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

var (
	DataFolder       = getEnv("DATA_FOLDER", "data")
	TestH265FileName = path.Join(DataFolder, `Big_Buck_Bunny_1080_10s_1MB.mp4`)
)

func init() {
	// Initialize GStreamer
	gst.Init(nil)
}

func TestSegmentPipelineFromFile(t *testing.T) {
	_, err := os.Stat(TestH265FileName)
	assert.Nil(t, err)
	// setup pipeline
	source := NewFileSrcElement("source", TestH265FileName)
	decoded := NewDecodeElement("bin")
	pipeline, err := NewSegmentPipeline("segment-pipeline", SegmentPipelineParams{videoDuration: time.Second}, decoded, source)
	assert.Nil(t, err)

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
		ctx.Done()
	}()

	select {
	case <-ctx.Done():
		break
	}
	loop.Quit()

	assert.Equal(t, pipeline.SegmentCounter, uint32(10))
}
