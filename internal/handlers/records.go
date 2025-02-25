package handlers

import (
	"brewing-temperature-monitor-app/internal/helpers"
	"brewing-temperature-monitor-app/internal/models"
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api" // Use v2
)

var supportedAggregations = []string{"sum", "max", "mean", "min"}
var aggregationsMapper = map[string]string{
	"sum":  "sum()",
	"max":  "max()",
	"mean": "mean()",
	"min":  "min()",
}

type QueryParams struct {
	Start   string `form:"start" example:"-30d"`                // Start time for the query (e.g., "-30d")
	Stop    string `form:"stop" example:"2023-10-01T00:00:00Z"` // Stop time for the query (e.g., "2023-10-01T00:00:00Z")
	Aggr    string `form:"aggr" example:"mean"`                 // Aggregation function (e.g., "mean"), or mean,sum,max
	AggFreq string `form:"aggFreq" example:"1d"`                // Aggregation frequency (e.g., "1d")
}

// RecordHandler holds the InfluxDB client
type RecordHandler struct {
	InfluxClient influxdb2.Client
	Org          string
	Bucket       string
}

// execureQuery is a helper method that executes queries
func executeQuery(h *RecordHandler, query string, ctx context.Context) (*api.QueryTableResult, error) {
	queryAPI := h.InfluxClient.QueryAPI(h.Org)

	fmt.Printf("QUERY: %v\n", query)

	result, err := queryAPI.Query(ctx, query)
	return result, err
}

func NewRecordHandler(client influxdb2.Client, org, bucket string) *RecordHandler {
	return &RecordHandler{
		InfluxClient: client,
		Org:          org,
		Bucket:       bucket,
	}
}

// PostData handles sensor data submissions
// @Summary Submit sensor data
// @Description Submit sensor data (temperature, humidity, location) for a specific device
// @Tags records
// @Accept json
// @Produce json
// @Param data body models.SensorData true "Sensor data to submit"
// @Success 200 {object} map[string]interface{} "message: Data stored successfully, data: submitted data"
// @Failure 400 {object} map[string]string "error: Invalid input or humidity out of range"
// @Failure 500 {object} map[string]string "error: Failed to store data"
// @Router /records [post]
func (h *RecordHandler) PostData(c *gin.Context) {
	var data models.SensorData

	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if data.Humidity < 0 || data.Humidity > 100.0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Humidity Must be in range [0.0, 100.00]"})
		return

	}

	point := influxdb2.NewPointWithMeasurement("sensor_data").
		AddTag("device_id", data.DeviceID).
		AddField("temperature", data.Temperature).
		AddField("humidity", data.Humidity).
		AddField("location", data.Location).
		SetTime(data.TimestampSampled)

	writeAPI := h.InfluxClient.WriteAPIBlocking(h.Org, h.Bucket)
	if err := writeAPI.WritePoint(context.Background(), point); err != nil {
		log.Printf("Error writing to database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Data stored successfully", "data": data})
}

// GetAllData fetches all records from the database
// @Summary Get all sensor data
// @Description Retrieve all sensor data within a specified time range
// @Tags records
// @Produce json
// @Param start query string false "Start time for the query (e.g., '-30d')" default(-30d)
// @Param stop query string false "Stop time for the query (e.g., '2023-10-01T00:00:00Z'), if left empty gets current datetime" default()
// @Success 200 {object} map[string]interface{} "data: List of sensor records"
// @Failure 400 {object} map[string]string "error: Invalid query parameters"
// @Failure 500 {object} map[string]string "error: Failed to retrieve data"
// @Router /records [get]
func (h *RecordHandler) GetAllData(c *gin.Context) {

	params := QueryParams{
		Start: "-30d",
		Stop:  time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}

	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(400, gin.H{"error": "Invalid query parameters"})
		return
	}

	query := fmt.Sprintf(`
	from(bucket: "`+h.Bucket+`")
	|> range(start: %s, stop: %s)
	|> filter(fn: (r) => r._measurement == "sensor_data") 
	  |> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")
`, params.Start, params.Stop)

	fmt.Printf("QUERY: %v\n", query)

	queryAPI := h.InfluxClient.QueryAPI(h.Org)
	result, err := queryAPI.Query(context.Background(), query)
	if err != nil {
		log.Printf("Error querying data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve data"})
		return
	}

	var records []map[string]interface{}
	for result.Next() {
		values := result.Record().Values()

		record := map[string]interface{}{
			"timestampSampled": values["_time"],
			"deviceId":         values["device_id"],
			"temperature":      values["temperature"],
			"humidity":         values["humidity"],
			"location":         values["location"],
		}

		records = append(records, record)
	}

	if result.Err() != nil {
		log.Printf("Query error: %v", result.Err())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": records})
}

// GetDataFromDeviceByID retrieves data for a specific device
// @Summary Get sensor data by device ID
// @Description Retrieve sensor data for a specific device within a specified time range
// @Tags records
// @Produce json
// @Param deviceId path string true "Device ID"
// @Param start query string false "Start time for the query (e.g., '-30d')" default(-30d)
// @Param stop query string false "Stop time for the query (e.g., '2023-10-01T00:00:00Z'), if left empty gets current datetime" default()
// @Param aggr query string false "Aggregation function (e.g., 'mean', or mean,max,min for multiple aggregations)"
// @Param aggFreq query string false "Aggregation frequency (e.g., '1d')" default(1d)
// @Success 200 {object} map[string]interface{} "data: List of sensor records"
// @Failure 400 {object} map[string]string "error: Invalid query parameters or missing deviceId"
// @Failure 500 {object} map[string]string "error: Failed to retrieve data"
// @Router /records/devices/{deviceId} [get]s
func (h *RecordHandler) GetDataFromDeviceByID(c *gin.Context) {
	deviceId := c.Param("deviceId") // Extract deviceId from URL path

	params := QueryParams{
		Start:   "-30d",
		Stop:    time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		Aggr:    "",
		AggFreq: "1d",
	}

	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters"})
		return
	}

	var query string
	if params.Aggr == "" {
		query = fmt.Sprintf(`
			from(bucket: "%s")
			|> range(start: %s, stop: %s)
			|> filter(fn: (r) => r._measurement == "sensor_data")
			|> filter(fn: (r) => r["device_id"] == "%s")
			|> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")
		`, h.Bucket, params.Start, params.Stop, deviceId)
	} else {
		query = h.buildAggregatedQuery(deviceId, params)
	}

	records, err := h.executeAndProcessQuery(c, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": records})
}

// GetDataFromDeviceByLocation retrieves data for a specific location
// @Summary Get sensor data by location
// @Description Retrieve sensor data for a specific location within a specified time range
// @Tags records
// @Produce json
// @Param location path string true "Location"
// @Param start query string false "Start time for the query (e.g., '-30d')" default(-30d)
// @Param stop query string false "Stop time for the query (e.g., '2023-10-01T00:00:00Z'), if left empty gets current datetime" default()
// @Success 200 {object} map[string]interface{} "data: List of sensor records"
// @Failure 400 {object} map[string]string "error: Invalid query parameters or missing location"
// @Failure 500 {object} map[string]string "error: Failed to retrieve data"
// @Router /records/locations/{location} [get]
func (h *RecordHandler) GetDataFromDeviceByLocation(c *gin.Context) {

	location := c.Param("location") // Extract deviceId from URL path

	params := QueryParams{
		Start: "-30d",
		Stop:  time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}

	// Bind query parameters to the struct
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(400, gin.H{"error": "Invalid query parameters"})
		return
	}

	// Validate input
	if location == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "location parameter is required"})
		return
	}

	// Construct Flux query to retrieve all data for the given device
	query := fmt.Sprintf(`
		from(bucket: "`+h.Bucket+`")
		|> range(start: %s, stop: %s) 
		|> filter(fn: (r) => r._measurement == "sensor_data")
  		|> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")
		|> filter(fn: (r) => r["location"] == "%s")
	`, params.Start, params.Stop, location)

	fmt.Printf("QUERY: %v\n", query)

	queryAPI := h.InfluxClient.QueryAPI(h.Org)
	result, err := queryAPI.Query(context.Background(), query)
	if err != nil {
		log.Printf("Error querying data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve data"})
		return
	}

	var records []map[string]interface{}

	for result.Next() {
		values := result.Record().Values()

		record := map[string]interface{}{
			"timestampSampled": values["_time"],
			"deviceId":         values["device_id"],
			"temperature":      values["temperature"],
			"humidity":         values["humidity"],
			"location":         values["location"],
		}

		records = append(records, record)
	}

	if result.Err() != nil {
		log.Printf("Query error: %v", result.Err())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": records})
}

func (h *RecordHandler) buildAggregatedQuery(deviceID string, params QueryParams) string {
	var queryBuilder strings.Builder

	// Base query
	queryBuilder.WriteString(fmt.Sprintf(`
		data = from(bucket: "%s")
			|> range(start: %s, stop: %s)
			|> filter(fn: (r) => r._measurement == "sensor_data")
			|> filter(fn: (r) => r["device_id"] == "%s")
			|> filter(fn: (r) => r._field == "temperature" or r._field == "humidity")
	`, h.Bucket, params.Start, params.Stop, deviceID))

	aggrFunctions := strings.Split(params.Aggr, ",")

	// Single aggregation case
	if len(aggrFunctions) == 1 {
		aggFunc := aggrFunctions[0]
		queryBuilder.WriteString(fmt.Sprintf(`
			%s_data = data
				|> aggregateWindow(every: %s, fn: %s, createEmpty: false)
				|> set(key: "_aggregate", value: "%s")
				|> map(fn: (r) => ({ r with _field: "%s_${r._field}" }))
			%s_data
				|> pivot(rowKey: ["_time"], columnKey: ["_field"], valueColumn: "_value")
		`, aggFunc, params.AggFreq, aggFunc, aggFunc, aggFunc, aggFunc))
	} else {
		// Multiple aggregations case
		for _, aggFunc := range aggrFunctions {
			queryBuilder.WriteString(fmt.Sprintf(`
				%s_data = data
					|> aggregateWindow(every: %s, fn: %s, createEmpty: false)
					|> set(key: "_aggregate", value: "%s")
					|> map(fn: (r) => ({ r with _field: "%s_${r._field}" }))
			`, aggFunc, params.AggFreq, aggFunc, aggFunc, aggFunc))
		}

		queryBuilder.WriteString(fmt.Sprintf(`
			union(tables: [%s])
				|> pivot(rowKey: ["_time"], columnKey: ["_field"], valueColumn: "_value")
		`, strings.Join(aggrFunctions, "_data,")+"_data"))
	}

	return queryBuilder.String()
}

func (h *RecordHandler) executeAndProcessQuery(c *gin.Context, query string) ([]map[string]interface{}, error) {
	queryAPI := h.InfluxClient.QueryAPI(h.Org)
	result, err := queryAPI.Query(context.Background(), query)
	if err != nil {
		log.Printf("Error querying data: %v", err)
		return nil, err
	}

	var records []map[string]interface{}
	for result.Next() {
		values := result.Record().Values()

		// Extract only the fields we want in the final payload
		record := make(map[string]interface{})
		record["timestamp"] = values["_time"]
		record["device_id"] = values["device_id"]

		// Dynamically add aggregated fields (e.g., max_temperature, min_humidity, etc.)
		for key, value := range values {
			if helpers.IsInArray(strings.Split(key, "_")[0], supportedAggregations) {
				record[key] = value
			}
			// if strings.HasPrefix(key, "max_") || strings.HasPrefix(key, "min_") || strings.HasPrefix(key, "mean_") || strings.HasPrefix(key, "sum_") {
			// 	record[key] = value
			// }
		}

		records = append(records, record)
	}

	if result.Err() != nil {
		log.Printf("Query error: %v", result.Err())
		return nil, result.Err()
	}

	return records, nil
}

func RegisterRoutes(router *gin.Engine, recordHandler *RecordHandler) {
	router.POST("/records", recordHandler.PostData)
	router.GET("/records", recordHandler.GetAllData)
	router.GET("/records/devices/:deviceId", recordHandler.GetDataFromDeviceByID)
	router.GET("/records/locations/:location", recordHandler.GetDataFromDeviceByLocation)
}
