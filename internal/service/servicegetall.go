package service

import (
	"fmt"
	"sort"
)

func (s *Service) GetAllMetrics() ([]string, map[string]string) {
	result := make(map[string]string)
	keys := make([]string, 0, len(result))
	for key, gauge := range (*s.store).GetGaugeMap() {
		result[key] = fmt.Sprintf("%v", gauge)
		keys = append(keys, key)
	}
	for key, counter := range (*s.store).GetCounterMap() {
		result[key] = fmt.Sprintf("%v", counter)
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, result
}

func (s *Service) GetUpdateUrls(host string, port string) []string {
	urls := []string{}
	gaugeMap := (*s.store).GetGaugeMap()
	counterMap := (*s.store).GetCounterMap()
	for key, value := range gaugeMap {
		url := fmt.Sprintf("http://%s%s/update/gauge/%s/%v", host, port, key, value)
		urls = append(urls, url)
	}
	for key, value := range counterMap {
		url := fmt.Sprintf("http://%s%s/update/counter/%s/%v", host, port, key, value)
		urls = append(urls, url)
	}
	return urls
}
