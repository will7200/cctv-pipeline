package pipeline

import (
	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	DataFolder       = os.Getenv("DATA_FOLDER")
	TestH265FileName = path.Join(DataFolder, `Big_Buck_Bunny_1080_10s_1MB.mp4`)
)

func init() {
	// Initialize GStreamer
	gst.Init(nil)
}

func TestIngestPipelineFromFile(t *testing.T) {
	source := NewFileSrcElement("source", TestH265FileName)
	decoded := NewDecodeElement("bin")
	assert.Nil(t, source.Build())
	assert.Nil(t, decoded.Build())
	assert.Nil(t, source.Link(decoded))
	pipeline, err := NewIngestPipelineFromSource("ingest-pipeline", decoded)
	assert.Nil(t, err)
	loop := glib.NewMainLoop(glib.MainContextDefault(), false)
	go func() {
		err = pipeline.Start(loop)
		assert.Nil(t, err)
	}()

	time.Sleep(5 * time.Second)

	loop.Quit()
}
