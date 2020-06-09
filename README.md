# Temperature Census

## Overview
This project polls weather information and store it as metric using opencensus and prometheus.


### Spin-up Images
1. Navigate to deployment folder
`cd deployment`

2. Start Prometheus, InfluxDB and Grafana
`docker-compose up`
* Prometheus is now hosted on [localhost:9090](localhost:9090).


### Running Application
1. Navigate to src folder
`cd src`

2. Run main.go file
`go run main.go`
* Application will start polling weather information and export the metrics to prometheus 



_More detail to be added in the future._


