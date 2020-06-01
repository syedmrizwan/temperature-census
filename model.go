package main

type TemperatureDetail struct {
	Temperature float64 `json:"temperature"`
	FellsLike   float64 `json:"feels_like"`
	Pressure    float64 `json:"pressure"`
	Humidity    float64 `json:"humidity"`
}

type TemperatureData struct {
	TemperatureDetail TemperatureDetail
	Err               error
}

