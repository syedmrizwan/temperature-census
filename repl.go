package main

import (
	"bufio"
	"bytes"
	"context"
	"contrib.go.opencensus.io/exporter/prometheus"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	//Temperature detail measures
	MTemperature = stats.Float64("temperature", "The temperature in Fahrenheit", "F")

	MFeelsLike = stats.Float64("feels_like", "The temperature feel like in Fahrenheit", "F")

	// The latency in milliseconds
	MLatencyMs = stats.Float64("repl/latency", "The latency in milliseconds per REPL loop", "ms")

	// Counts/groups the lengths of lines read in.
	MLineLengths = stats.Int64("repl/line_lengths", "The distribution of line lengths", "By")
)

var (
	KeyMethod, _ = tag.NewKey("method")
	KeyStatus, _ = tag.NewKey("status")
	KeyError, _  = tag.NewKey("error")
)

var (
	TemperatureView = &view.View{
		Name:        "demo/temperature",
		Measure:     MTemperature,
		Description: "Temperature of Islamabad",
		Aggregation: view.LastValue(),
	}

	FeelLikeView = &view.View{
		Name:        "demo/feel_like",
		Measure:     MFeelsLike,
		Description: "Temperature of Islamabad",
		Aggregation: view.LastValue(),
	}

	LatencyView = &view.View{
		Name:        "demo/latency",
		Measure:     MLatencyMs,
		Description: "The distribution of the latencies",

		// Latency in buckets:
		// [>=0ms, >=25ms, >=50ms, >=75ms, >=100ms, >=200ms, >=400ms, >=600ms, >=800ms, >=1s, >=2s, >=4s, >=6s]
		Aggregation: view.Distribution(0, 25, 50, 75, 100, 200, 400, 600, 800, 1000, 2000, 4000, 6000),
		TagKeys:     []tag.Key{KeyMethod}}

	LineCountView = &view.View{
		Name:        "demo/lines_in",
		Measure:     MLineLengths,
		Description: "The number of lines from standard input",
		Aggregation: view.LastValue(),
	}

	LineLengthView = &view.View{
		Name:        "demo/line_lengths",
		Description: "Groups the lengths of keys in buckets",
		Measure:     MLineLengths,
		// Lengths: [>=0B, >=5B, >=10B, >=15B, >=20B, >=40B, >=60B, >=80, >=100B, >=200B, >=400, >=600, >=800, >=1000]
		Aggregation: view.Distribution(0, 5, 10, 15, 20, 40, 60, 80, 100, 200, 400, 600, 800, 1000),
	}
)

func main() {
	// Register the views, it is imperative that this step exists
	// lest recorded metrics will be dropped and never exported.
	if err := view.Register(LatencyView, LineCountView, LineLengthView, TemperatureView, FeelLikeView); err != nil {
		log.Fatalf("Failed to register the views: %v", err)
	}

	// Create the Prometheus exporter.
	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: "exporter",
	})
	if err != nil {
		log.Fatalf("Failed to create the Prometheus stats exporter: %v", err)
	}

	// Now finally run the Prometheus exporter as a scrape endpoint.
	// We'll run the server on port 8888.
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", pe)
		if err := http.ListenAndServe(":8888", mux); err != nil {
			log.Fatalf("Failed to run Prometheus scrape endpoint: %v", err)
		}
	}()

	doEvery(2*time.Second, getTemperatureDetail)
	// In a REPL:
	//   1. Read input
	//   2. process input
	//br := bufio.NewReader(os.Stdin)

	// repl is the read, evaluate, print, loop
	//for {
	//	if err := readEvaluateProcess(br); err != nil {
	//		if err == io.EOF {
	//			return
	//		}
	//		log.Fatal(err)
	//	}
	//}
}

// readEvaluateProcess reads a line from the input reader and
// then processes it. It returns an error if any was encountered.
func readEvaluateProcess(br *bufio.Reader) (terr error) {
	ctx, err := tag.New(context.Background(), tag.Insert(KeyMethod, "repl"), tag.Insert(KeyStatus, "OK"))
	if err != nil {
		return err
	}
	startTime := time.Now()
	defer func() {
		if terr != nil {
			ctx, _ = tag.New(ctx, tag.Upsert(KeyStatus, "ERROR"),
				tag.Upsert(KeyError, terr.Error()))
		}

		stats.Record(ctx, MLatencyMs.M(sinceInMilliseconds(startTime)))
	}()

	fmt.Printf("> ")
	line, _, err := br.ReadLine()
	if err != nil {
		if err != io.EOF {
			return err
		}
		log.Fatal(err)
	}

	out, err := processLine(ctx, line)

	if err != nil {
		return err
	}
	fmt.Printf("< %s\n\n", out)
	return nil
}

// processLine takes in a line of text and
// transforms it. Currently it just capitalizes it.
func processLine(ctx context.Context, in []byte) (out []byte, err error) {
	startTime := time.Now()
	metricValue := int64(len(in))

	if bytes.Equal(in, []byte("UNREGISTER")) {
		view.Unregister(LineCountView)
		view.Register(LineCountView)
		return bytes.ToUpper(in), nil
	} else if bytes.Equal(in, []byte("NEGATIVE")) {
		metricValue = -15
	}

	defer func() {
		stats.Record(ctx, MLatencyMs.M(sinceInMilliseconds(startTime)),
			MLineLengths.M(metricValue))
	}()

	return bytes.ToUpper(in), nil
}

func sinceInMilliseconds(startTime time.Time) float64 {
	return float64(time.Since(startTime).Nanoseconds()) / 1e6
}

func getTemperatureDetail() (*TemperatureDetail, error) {
	client := resty.New()
	url := "http://api.openweathermap.org/data/2.5/weather?id=1162015&APPID=bfecbb35de69974dc8e56c12f3a90801"
	resp, err := client.R().Get(url)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	var tempDetail TemperatureDetail
	tempDetail.Temperature = gjson.Get(string(resp.Body()), `main.temp`).Float()
	tempDetail.FeelsLike = gjson.Get(string(resp.Body()), `main.feels_like`).Float()
	tempDetail.Pressure = gjson.Get(string(resp.Body()), `main.pressure`).Float()
	tempDetail.Humidity = gjson.Get(string(resp.Body()), `main.humidity`).Float()
	fmt.Println(tempDetail.Temperature)

	defer func() {
		stats.Record(context.Background(), MTemperature.M(tempDetail.Temperature), MFeelsLike.M(tempDetail.FeelsLike))
	}()

	return &tempDetail, nil
}

func doEvery(d time.Duration, f func() (*TemperatureDetail, error)) {
	for range time.Tick(d) {
		f()
	}
}
