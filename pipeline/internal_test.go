package pipeline

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	description2 "github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/go-gst/go-gst/gst"
	"github.com/stretchr/testify/assert"
)

func TestCCTVPipeline(tt *testing.T) {
	tests := []struct {
		encoding     string
		location     string
		sinkLocation string
	}{
		{`x264enc`, `rtsp://localhost:9000/stream`, `rtsp://localhost:9001/stream`},
		//{`x264enc`, `rtsp://localhost:9002/stream`, `rtsp://localhost:9003/stream`},
	}

	for _, test := range tests {
		tt.Run(test.encoding, func(t *testing.T) {

			sinkListen, err := url.Parse(test.sinkLocation)
			assert.Nil(t, err)
			listen, err := url.Parse(test.location)
			assert.Nil(t, err)
			sourceListenAddress := fmt.Sprintf(":%s", listen.Port())
			sinkListenAddress := fmt.Sprintf(":%s", sinkListen.Port())

			cctvPipeline, err := NewCCTVPipeline(CCTVPipelineParams{
				SourceLocation:                  test.location,
				DestinationLocation:             test.sinkLocation,
				CameraID:                        TestCameraID,
				SegmentBasePath:                 newSegmentBase(),
				TargetVideoSegmentationDuration: time.Second,
			})
			assert.Nil(t, err)

			// setup context
			waitFor := time.Second * 10
			ctx := context.Background()
			ctx, cancel := context.WithTimeout(ctx, waitFor)
			time.AfterFunc(waitFor, cancel)

			server := setupFakeRTSP(ctx, sourceListenAddress)
			server2 := setupFakeRTSP(ctx, sinkListenAddress)
			_ = streamToRTSP(ctx, test.encoding, test.location)
			for {
				time.Sleep(50 * time.Millisecond)
				if server.HasStream() {
					break
				}
			}

			err = cctvPipeline.Start(ctx)
			assert.Nil(t, err)
			go DebugGSTState(ctx, cctvPipeline.basePipeline, fmt.Sprintf("%s.%s", "base", test.encoding))
			go DebugGSTState(ctx, cctvPipeline.thumbnailPipeline, fmt.Sprintf("%s.%s", "thumbnail", test.encoding))

			var desp *description2.Session
			hasStream := false
			go func() {
			outer:
				for {
					select {
					case <-ctx.Done():
						break outer
					default:
						time.Sleep(50 * time.Millisecond)
						if server2.HasStream() {
							hasStream = true
							desp = server2.StreamDescription()
							cancel()
							break outer
						}
					}
				}
			}()

			select {
			case <-ctx.Done():
				break
			}
			time.Sleep(1 * time.Second)
			err = cctvPipeline.Stop()
			assert.Nil(t, err)

			assert.Equal(t, true, hasStream, "Target sink never received a stream")
			if !hasStream {
				assert.FailNow(t, "no stream was provided")
			}
			assert.NotNil(t, desp)
			assert.Equal(t, desp.Medias[0].Formats[0].Codec(), "MPEG-TS")
		})
	}
}

func DebugGSTState(ctx context.Context, pipeline *Pipeline, output string) {
outer:
	for {
		pipeline.pipeline.DebugBinToDotFileWithTs(gst.DebugGraphShowAll, output)
		select {
		case <-ctx.Done():
			break outer
		case <-time.After(time.Second * 1):
			break
		}
	}
	pipeline.pipeline.DebugBinToDotFileWithTs(gst.DebugGraphShowAll, output)
}
