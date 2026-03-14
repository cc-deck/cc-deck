// Package main implements a simple weather CLI tool.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
)

type Weather struct {
	City      string
	Temp      int
	Condition string
}

var conditions = []string{"Sunny", "Cloudy", "Rainy", "Snowy", "Windy"}

func getWeather(city string) Weather {
	// Simulated weather data based on city name hash
	hash := 0
	for _, c := range city {
		hash += int(c)
	}
	r := rand.New(rand.NewSource(int64(hash)))

	return Weather{
		City:      city,
		Temp:      r.Intn(35) - 5,
		Condition: conditions[r.Intn(len(conditions))],
	}
}

func formatWeather(w Weather) string {
	return fmt.Sprintf("Weather for %s:\n  Temperature: %d°C\n  Condition: %s",
		w.City, w.Temp, w.Condition)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: weather-cli <city> [city2] ...")
		os.Exit(1)
	}

	cities := os.Args[1:]
	for i, city := range cities {
		city = strings.TrimSpace(city)
		w := getWeather(city)
		fmt.Println(formatWeather(w))
		if i < len(cities)-1 {
			fmt.Println()
		}
	}
}
