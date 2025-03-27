package main

import (
	"net/http"
	"strconv"
	"strings"
)

type memStorage struct {
	gauge   map[string]int
	counter map[string]float64
}

var storageMetrics = new(memStorage)

func gaugeParse(res http.ResponseWriter, key string, value int) {
	storageMetrics.gauge[key] = value
	res.WriteHeader(http.StatusOK)
}

func counterParse(res http.ResponseWriter, key string, value float64) {
	storageMetrics.counter[key] += value
	res.WriteHeader(http.StatusOK)
}

// func printHttpFuncDebug(res http.ResponseWriter) {
// 	body := "Header ===============\r\n"
// 	for k, v := range res.Header() {
// 		body += fmt.Sprintf("%s: %v\r\n", k, v)
// 	}

// 	body += fmt.Sprintf("%+v\r\n", "storageMetrics.gauge")
// 	for k, v := range storageMetrics.gauge {
// 		body += fmt.Sprintf("%+v: %+v\r\n", k, v)
// 	}
// 	body += fmt.Sprintf("%+v\r\n", "storageMetrics.counter")
// 	for k, v := range storageMetrics.counter {
// 		body += fmt.Sprintf("%+v: %+v\r\n", k, v)
// 	}
// 	res.Write([]byte(body))
// }

func updateMetrics(res http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		path := strings.Split(req.URL.Path, "/")[1:]
		if len(path) != 4 {
			res.WriteHeader(http.StatusNotFound)
		} else if path[0] == "update" {
			if path[1] == "gauge" {
				value, err := strconv.Atoi(path[3])
				if err != nil {
					res.WriteHeader(http.StatusBadRequest)
				} else {
					gaugeParse(res, path[2], value)
				}
			} else if path[1] == "counter" {
				value, err := strconv.ParseFloat(path[3], 64)
				if err != nil {
					res.WriteHeader(http.StatusBadRequest)
				} else {
					counterParse(res, path[2], value)
				}
			} else {
				res.WriteHeader(http.StatusBadRequest)
			}
		}
		// printHttpFuncDebug(res)

	}
}

func main() {
	storageMetrics.gauge = make(map[string]int)
	storageMetrics.counter = make(map[string]float64)
	http.HandleFunc(`/update/`, updateMetrics)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}
