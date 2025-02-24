package models

import "time"

// SensorData represents the structure of a sensor record
type SensorData struct {
	DeviceID         string    `json:"deviceId" binding:"required"`
	Temperature      float64   `json:"temperature" binding:"required"`
	Humidity         float64   `json:"humidity" binding:"required"`
	Location         string    `json:"location" binding:"required"`
	TimestampSampled time.Time `json:"timestampSampled" binding:"required"` // the timestamp taht the sample was taken by device
}
