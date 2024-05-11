package pipeline

import "context"

type PartialPipeline interface {
	// Prepare is called before any elements are built or any gst
	// pipeline has be instantiated
	Prepare(pipeline *Pipeline) error
	// Build is called after all elements have been added to the pipeline
	// gst pipeline has been instantiated
	Build(pipeline *Pipeline) error
	// Connect is called to connect a target element from one segment to
	// another
	Connect(source *Element) error
	// Start is called when the pipeline has started processing
	Start(ctx context.Context, pipeline *Pipeline) error
	// Stop is called when the pipeline has finished processing
	Stop(ctx context.Context, pipeline *Pipeline) error
}
