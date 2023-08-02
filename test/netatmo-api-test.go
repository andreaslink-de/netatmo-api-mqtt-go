package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	toml "github.com/BurntSushi/toml"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	netatmo "github.com/joshuabeny1999/netatmo-api-go/v2"
)

/*
// Prepare with:
go build test/netatmo-api-test.go

// Generate Netatmo app data, create new netatmo.conf and enter params following sample.conf

// Test via:
go build test/netatmo-api-test.go -f test/netatmo.conf

// Run compiled binary as command line call via e.g. cron:
./netatmo-api-test -f test/netatmo.conf
*/

// Command line flag
var fConfig = flag.String("f", "", "Configuration file")

type NetatmoConfig struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
}

var config NetatmoConfig
var mqttClient MQTT.Client

func initializeMqttClient(opts *MQTT.ClientOptions) {
	mqttClient = MQTT.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
}

func main() {

	// Parse command line flags
	flag.Parse()
	if *fConfig == "" {
		fmt.Printf("Missing required argument -f\n")
		os.Exit(0)
	}

	if _, err := toml.DecodeFile(*fConfig, &config); err != nil {
		fmt.Printf("Cannot parse config file: %s\n", err)
		os.Exit(1)
	}

	n, err := netatmo.NewClient(netatmo.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RefreshToken: config.RefreshToken,
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	dc, err := n.Read()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ct := time.Now().UTC().Unix()

	// Prepare MQTT Client >>>>>>>>>>>>>>>>>>>>>>>
	// Set up MQTT client options
	opts := MQTT.NewClientOptions().AddBroker("tcp://192.168.42.253:1883") // Replace with your MQTT broker's address

	// Initialize and connect MQTT client
	initializeMqttClient(opts)
	// End MQTT preparation <<<<<<<<<<<<<<<<<<<<<

	for _, station := range dc.Stations() {
		fmt.Printf("Station: %s [%s]\n", station.StationName, station.ID)
		fmt.Printf("\tCity: %s\n\tCountry: %s\n\tTimezone: %s\n\tLongitude: %f\n\tLatitude: %f\n\tAltitude: %d\n\n", station.Place.City, station.Place.Country, station.Place.Timezone, *station.Place.Location.Longitude, *station.Place.Location.Latitude, *station.Place.Altitude)

		for _, module := range station.Modules() {
			fmt.Printf("\tModule: %s [%s]\n", module.ModuleName, module.ID)

			{
				if module.DashboardData.LastMeasure == nil {
					fmt.Printf("\t\tSkipping %s, no measurement data available.\n", module.ModuleName)
					continue
				}
				ts, data := module.Info()
				for dataName, value := range data {
					fmt.Printf("\t\t%s : %v (updated %ds ago)\n", dataName, value, ct-ts)
				}
			}

			{
				ts, data := module.Data()
				for dataName, value := range data {
					fmt.Printf("\t\t%s : %v (updated %ds ago)\n", dataName, value, ct-ts)
					checkModuleToSend(module.ID, dataName, fmt.Sprint(value))
				}
			}
		}
	}

	//MQTT shutdown
	// Wait for a few seconds before closing the connection
	time.Sleep(2 * time.Second)

	// Disconnect from the MQTT broker
	mqttClient.Disconnect(250)
}

// Evaluate value and send
func checkModuleToSend(moduleID string, dataName string, value string) {
	//Wohnzimmer
	if moduleID == "70:ee:50:00:e3:96" {
		//fmt.Printf("%s >>>>> %s : %s \n", moduleID, dataName, value)

		if dataName == "Temperature" {
			sendValueToMQTT("zuhause/haus/wohnzimmer/raum/temperatur", value)
		}

		if dataName == "Humidity" {
			sendValueToMQTT("zuhause/haus/wohnzimmer/raum/humidity", value)
		}

		if dataName == "CO2" {
			sendValueToMQTT("zuhause/haus/wohnzimmer/raum/co2", value)
		}

		if dataName == "Noise" {
			sendValueToMQTT("zuhause/haus/wohnzimmer/raum/noise", value)
		}

		if dataName == "Pressure" {
			sendValueToMQTT("zuhause/haus/wohnzimmer/raum/pressure", value)
		}
	}

	//Aussen
	if moduleID == "02:00:00:00:d1:ac" {
		if dataName == "Temperature" {
			sendValueToMQTT("zuhause/garten/netatmo/aussen/temperatur", value)
		}

		if dataName == "Humidity" {
			sendValueToMQTT("zuhause/garten/netatmo/aussen/humidity", value)
		}
	}

	//Schlafzimmer
	if moduleID == "03:00:00:03:64:22" {
		if dataName == "Temperature" {
			sendValueToMQTT("zuhause/haus/schlafzimmer/raum/temperatur", value)
		}

		if dataName == "Humidity" {
			sendValueToMQTT("zuhause/haus/schlafzimmer/raum/humidity", value)
		}

		if dataName == "CO2" {
			sendValueToMQTT("zuhause/haus/schlafzimmer/raum/co2", value)
		}
	}

	//Kinderzimmer
	if moduleID == "03:00:00:02:43:34" {
		if dataName == "Temperature" {
			sendValueToMQTT("zuhause/haus/kinderzimmer/raum/temperatur", value)
		}

		if dataName == "Humidity" {
			sendValueToMQTT("zuhause/haus/kinderzimmer/raum/humidity", value)
		}

		if dataName == "CO2" {
			sendValueToMQTT("zuhause/haus/kinderzimmer/raum/co2", value)
		}
	}
}

// Send values via MQTT
func sendValueToMQTT(topic string, value string) {
	//fmt.Printf("'%s' --> '%s' \n\n", topic, value)

	token := mqttClient.Publish(topic, 0, false, value)
	token.Wait()

	if token.Error() != nil {
		fmt.Printf("ERROR: Failed to publish message to topic %s: %s\n", topic, token.Error())
	} else {
		fmt.Printf("\t\t\tPublished message '%s' to topic '%s'\n", value, topic)
	}
}
