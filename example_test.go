package gotopo_test

import (
	"context"
	"fmt"

	"github.com/jeremyrickard/gotopo"
)

func ExampleClient_GetFeatures() {
	client, err := gotopo.NewClient(gotopo.WithEndpoint("localhost:8080"))
	if err != nil {
		panic(err)
	}
	defer client.Close()

	// Opening a map performs network I/O, so real applications first call:
	// client.OpenMap(context.Background(), "ABCDE", gotopo.OpenMapOptions{})
	_, _ = client.GetFeatures(context.Background(), gotopo.FeatureFilter{Class: "Marker"})
	fmt.Println("context-aware feature query")
	// Output: context-aware feature query
}
