module github.com/will7200/cctver

go 1.22.0

require (
	github.com/bluenviron/gortsplib/v4 v4.9.0
	github.com/go-gst/go-glib v1.0.1
	github.com/go-gst/go-gst v1.0.0
	github.com/google/uuid v1.6.0
	github.com/joho/godotenv v1.5.1
	github.com/pion/rtp v1.8.7-0.20240429002300-bc5124c9d0d0
	github.com/rs/zerolog v1.32.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.9.0
	go.uber.org/multierr v1.11.0
	gopkg.in/vansante/go-ffprobe.v2 v2.1.1
)

require (
	github.com/bluenviron/mediacommon v1.10.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.14 // indirect
	github.com/pion/sdp/v3 v3.0.9 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/net v0.24.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/go-gst/go-glib => ./pkg/go-glib
