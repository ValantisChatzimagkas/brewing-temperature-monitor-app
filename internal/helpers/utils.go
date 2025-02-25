package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
)

// isInArray checks if an element is present
func IsInArray[T comparable](element T, array []T) bool {
	for _, el := range array {
		if element == el {
			return true
		}
	}
	return false
}

// fixArgument converts single digit values to double digit so that they can be part of a date, 1 -> 01
func fixArgument(arg int) string {

	if arg-10 < 0 {
		return fmt.Sprintf("0%v", arg)
	} else {
		return strconv.Itoa(arg)
	}
}

// GenerateDummyData is a helper function for generating and posting dummy data
func GenerateDummyData(url string) {

	var data []map[string]interface{}

	fmt.Printf("Generating data...")

	for d := 1; d < 10; d++ {
		for h := 0; h < 24; h++ {
			for m := 0; m <= 59; m += 15 {

				day, hour, minute := fixArgument(d), fixArgument(h), fixArgument(m)

				record := map[string]interface{}{
					"timestampSampled": fmt.Sprintf(`2024-02-%vT%v:%v:00Z`, day, hour, minute),
					"deviceId":         "sensor_123",
					"humidity":         0.0 + rand.Float64()*(100.0-0.0),
					"temperature":      14 + rand.Float64()*(21-14),
					"location":         "Room 1",
				}

				data = append(data, record)
			}
		}
	}
	// forward data
	for idx, record := range data {
		fmt.Printf("Record - %v: %v\n", idx, record)
		payloadBody, _ := json.Marshal(record)
		responseBody := bytes.NewBuffer(payloadBody)

		resp, err := http.Post(url, "application/json", responseBody)

		if err != nil {
			fmt.Printf("An error occurred: %v\n", err)
		}
		defer resp.Body.Close()
	}
}
