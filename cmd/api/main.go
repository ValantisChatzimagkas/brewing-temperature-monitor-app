package main

import (
	"brewing-temperature-monitor-app/internal/handlers"
	"brewing-temperature-monitor-app/internal/helpers"
	"log"
	"os"

	_ "brewing-temperature-monitor-app/docs"

	"github.com/gin-gonic/gin"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	gotdotenv "github.com/joho/godotenv"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

//https://azar-writes-blogs.vercel.app/post/Simplifying-Documentation:-Generate-Swagger-for-Your-Go-Gin-Server-Automatically-with-Swag-831649429ff24428bfaf3f59eb1ad83e

// @title Brewing Temperature Monitor API
// @version 1.0
// @description This is a sample API for monitoring brewing temperature and humidity data.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
func main() {
	// Load environment variables from .env file
	err := gotdotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Retrieve InfluxDB token from environment variables
	token := os.Getenv("INFLUX_TOKEN")
	if token == "" {
		log.Fatal("INFLUX_TOKEN environment variable is not set")
	}

	// Set up InfluxDB connection
	influxClient := influxdb2.NewClient("http://localhost:8086", token)
	defer influxClient.Close()

	// Retrieve InfluxDB organization and bucket from environment variables
	org := os.Getenv("INFLUX_ORG")
	bucket := os.Getenv("INFLUX_BUCKET")
	if org == "" || bucket == "" {
		log.Fatal("INFLUX_ORG or INFLUX_BUCKET environment variables are not set")
	}

	router := gin.Default()

	recordHandler := handlers.NewRecordHandler(influxClient, org, bucket)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	handlers.RegisterRoutes(router, recordHandler)

	go func() {
		if err := router.Run(":8080"); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	go helpers.GenerateDummyData("http://localhost:8080/records")

	select {}
}
