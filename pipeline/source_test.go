package pipeline

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/will7200/cctver/internal/rtsp"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func setupFakeRTSP(ctx context.Context, listen string) *rtsp.ServerHandler {
	server := rtsp.NewServerHandler(rtsp.ServerHandlerParams{
		RTSPAddress: listen,
	})

	go func() {
		log.Print("Starting RTSP server at localhost:9000")
		server.StartAndWait()
	}()

	go func() {
		select {
		case <-ctx.Done():
			break
		}
		server.Close()
	}()
	return server
}

func streamToRTSP(ctx context.Context, encoding string, location string) *glib.MainLoop {
	pipeline, err := gst.NewPipelineFromString(fmt.Sprintf(` videotestsrc 
! videoconvert ! videoscale ! video/x-raw,width=640,height=480 
! %s
! rtspclientsink protocols=GST_RTSP_LOWER_TRANS_TCP location=%s`, encoding, location))
	if err != nil {
		panic(err)
	}

	// start loop
	loop := glib.NewMainLoop(glib.MainContextDefault(), false)
	go func() {
		err = runGSTPipeline(loop, pipeline)
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
			break
		}
		loop.Quit()
	}()
	return loop
}

func TestSourcePipeline(t *testing.T) {

	tests := []struct {
		encoding      string
		location      string
		listenAddress string
	}{{`x264enc`, `rtsp://localhost:9000/stream`, `:9000`}, {`x265enc`, `rtsp://localhost:9001/stream`, `:9001`}}
	for _, test := range tests {
		testname := fmt.Sprintf("%s-%s", "source_pipeline", test.encoding)
		t.Run(testname, func(t *testing.T) {
			source, err := NewSourcePipeline(SourcePipelineParams{RtspUri: test.location})
			assert.Nil(t, err)

			pipeline := NewPipeline("segment-pipeline")
			pipeline.AddPartialPipeline(source)
			assert.Nil(t, pipeline.Build())

			sink, err := gst.NewElementWithName("appsink", "customsink")
			assert.Nil(t, err)
			customSink := app.SinkFromElement(sink)
			index := 0
			customSink.SetCallbacks(&app.SinkCallbacks{
				NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
					sample := sink.PullSample()
					if sample == nil {
						return gst.FlowEOS
					}

					buffer := sample.GetBuffer()
					if buffer == nil {
						return gst.FlowError
					}
					index++
					if index > 100 {
						return gst.FlowEOS
					}
					return gst.FlowOK
				},
			})
			assert.Nil(t, pipeline.pipeline.AddMany(sink))
			assert.Nil(t, source.Elements.tee.el.Link(customSink.Element))
			customSink.SyncStateWithParent()

			// setup context
			waitFor := time.Second * 10
			ctx := context.Background()
			ctx, cancel := context.WithTimeout(ctx, waitFor)
			time.AfterFunc(waitFor, cancel)

			go func(ctx context.Context) {
				for {
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Second * 1):
						break
					}
					pipeline.pipeline.DebugBinToDotFileWithTs(gst.DebugGraphShowAll, test.encoding)
				}
			}(ctx)

			server := setupFakeRTSP(ctx, test.listenAddress)
			_ = streamToRTSP(ctx, test.encoding, test.location)
			for {
				time.Sleep(50 * time.Millisecond)
				if server.HasStream() {
					break
				}
			}

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
			assert.Greater(t, index, 100)
		})
	}
}
