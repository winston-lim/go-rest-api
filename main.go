package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type Geometry struct {
	Lat string
	Lon string
}

type Route struct {
	Name       string `json:"name"`
	Short_name string `json:"short_name"`
}

type Forecast struct {
	Forecast_seconds float64 `json:"forecast_seconds"`
	Route            Route   `json:"route"`
	Rv_id            int     `json:"rv_id"`
	Vehicle          string  `json:"vehicle"`
	Vehicle_id       int     `json:"vehicle_id"`
}

type Vehicle struct {
	Vehicle_id        int      `json:"vehicle_id"`
	Registration_code string   `json:"registration_code"`
	Speed             string   `json:"speed"`
	Position          Geometry `json:"position"`
	Stats             struct {
		Avg_speed string  `json:"avg_speed"`
		Speed     float64 `json:"speed"`
	} `json:"stats"`
}

type BusStopInfoResponse struct {
	Id       int        `json:"id"`
	Name     string     `json:"name"`
	Geometry []Geometry `json:"geometry"`
	Forecast []Forecast `json:"forecast"`
}

type BusLineInfoResponse struct {
	Id        int       `json:"id"`
	Name      string    `json:"name"`
	Routename string    `json:"routename"`
	Vehicles  []Vehicle `json:"vehicles"`
}

type BusLineArrivalForecast struct {
	Api_forecast     string `json:"api_forecast"`
	Average_forecast string `json:"average_forecast"`
	Current_forecast string `json:"current_forecast"`
	Distance         string `json:"distance"`
}

type BusLineInfo struct {
	Id                int                      `json:"id"`
	Name              string                   `json:"name"`
	Short_name        string                   `json:"short_name"`
	Arrival_forecasts []BusLineArrivalForecast `json:"arrival_forecasts"`
}

type BusStopInfo struct {
	Name      string        `json:"name"`
	Id        int           `json:"id"`
	Geometry  Geometry      `json:"geometry"`
	Bus_lines []BusLineInfo `json:"bus_lines"`
}

func distance(lat1 float64, lng1 float64, lat2 float64, lng2 float64, unit ...string) float64 {
	const PI float64 = 3.141592653589793

	radlat1 := float64(PI * lat1 / 180)
	radlat2 := float64(PI * lat2 / 180)

	theta := float64(lng1 - lng2)
	radtheta := float64(PI * theta / 180)

	dist := math.Sin(radlat1)*math.Sin(radlat2) + math.Cos(radlat1)*math.Cos(radlat2)*math.Cos(radtheta)

	if dist > 1 {
		dist = 1
	}

	dist = math.Acos(dist)
	dist = dist * 180 / PI
	dist = dist * 60 * 1.1515

	if len(unit) > 0 {
		if unit[0] == "K" {
			dist = dist * 1.609344
		} else if unit[0] == "N" {
			dist = dist * 0.8684
		}
	}

	return dist
}

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the HomePage!")
	fmt.Println("Endpoint Hit: homePage")
}

func returnBusStopInfo(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint Hit: returnBusStopInfo")
	// get all route variables
	vars := mux.Vars(r)
	// extract busStopId
	id := vars["id"]
	// fetch bus timing with busStopId
	response, err := http.Get("https://baseride.com/routes/api/platformbusarrival/" + id + "/?format=json")
	if err != nil {
		fmt.Println(err.Error())
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Println(err)
	}

	var busStopResponseObject BusStopInfoResponse
	json.Unmarshal(responseData, &busStopResponseObject)
	//json.NewEncoder(w).Encode(busStopResponseObject)
	// declare response object and update with bus stop information
	var responseObject BusStopInfo
	responseObject.Id = busStopResponseObject.Id
	responseObject.Name = busStopResponseObject.Name
	responseObject.Geometry = busStopResponseObject.Geometry[0]

	// create map of vehicle_id to forecast timings
	vehicleToForecasts := make(map[int]float64)
	for _, forecast := range busStopResponseObject.Forecast {
		vehicleToForecasts[forecast.Vehicle_id] = forecast.Forecast_seconds
	}

	// get unique route_ids
	route_ids := []int{}
	for _, fi := range busStopResponseObject.Forecast {
		contains := false
		for _, id := range route_ids {
			if id == fi.Rv_id {
				contains = true
			}
		}
		if !contains {
			route_ids = append(route_ids, fi.Rv_id)
		}
	}

	//for each unique busline, fetch vehicle information and update response.bus_lines
	for _, route_id := range route_ids {
		response, err := http.Get("https://baseride.com/routes/apigeo/routevariantvehicle/" + strconv.Itoa(route_id) + "/?format=json")
		if err != nil {
			fmt.Println(err.Error())
		}
		responseData, err := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Println(err)
		}

		var busLineResponseObject BusLineInfoResponse
		json.Unmarshal(responseData, &busLineResponseObject)

		var busLineInfo BusLineInfo
		busLineInfo.Id = busLineResponseObject.Id
		busLineInfo.Name = busLineResponseObject.Name
		busLineInfo.Short_name = busLineResponseObject.Routename

		// for each vehicle belonging to a line, update arrival forecasts
		for _, v := range busLineResponseObject.Vehicles {
			var arrival_Forecast BusLineArrivalForecast
			arrival_Forecast.Api_forecast = fmt.Sprintf("%f", vehicleToForecasts[v.Vehicle_id])
			stop_lat, _ := strconv.ParseFloat(responseObject.Geometry.Lat, 64)
			stop_lon, _ := strconv.ParseFloat(responseObject.Geometry.Lon, 64)
			vehicle_lat, _ := strconv.ParseFloat(v.Position.Lat, 64)
			vehicle_lon, _ := strconv.ParseFloat(v.Position.Lon, 64)
			calc_dist := distance(stop_lat, stop_lon, vehicle_lat, vehicle_lon, "K")
			avg_speed, _ := strconv.ParseFloat(v.Stats.Avg_speed, 64)
			if v.Stats.Speed == 0 || avg_speed == 0 {
				if v.Stats.Speed == 0 {
					arrival_Forecast.Current_forecast = "0"
				}
				if avg_speed == 0 {
					arrival_Forecast.Average_forecast = "0"
				}
			} else {
				current_forecast := calc_dist * 3600 / v.Stats.Speed
				average_forecast := calc_dist * 3600 / avg_speed
				arrival_Forecast.Current_forecast = fmt.Sprintf("%f", current_forecast)
				arrival_Forecast.Average_forecast = fmt.Sprintf("%f", average_forecast)
			}
			arrival_Forecast.Distance = fmt.Sprintf("%f", calc_dist)
			busLineInfo.Arrival_forecasts = append(busLineInfo.Arrival_forecasts, arrival_Forecast)
		}
		responseObject.Bus_lines = append(responseObject.Bus_lines, busLineInfo)
	}
	json.NewEncoder(w).Encode(responseObject)
}

func handleRequests() {
	// create new instance of mux router
	myRouter := mux.NewRouter().StrictSlash(true)

	myRouter.HandleFunc("/", homePage)
	myRouter.HandleFunc("/getBusStopInfo/{id}", returnBusStopInfo)

	//use myRouter
	log.Fatal(http.ListenAndServe(":10000", myRouter))
}

func main() {
	handleRequests()
}
