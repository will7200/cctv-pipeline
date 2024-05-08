module github.com/will7200/cctver

go 1.22.0

require (
	github.com/go-gst/go-glib v1.0.1
	github.com/go-gst/go-gst v1.0.0
	github.com/joho/godotenv v1.5.1
	github.com/rs/zerolog v1.32.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.9.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.12.0 // indirect
	gopkg.in/vansante/go-ffprobe.v2 v2.1.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/go-gst/go-glib => ./pkg/go-glib
