package pipeline

import (
	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
	"os"
	"path"
	"testing"

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
	pipeline, err := NewIngestPipelineFromSource("ingest-pipeline", source)
	assert.Nil(t, err)
	loop := glib.NewMainLoop(glib.MainContextDefault(), false)
	err = pipeline.Start(loop)
	assert.Nil(t, err)
}
