package service

import (
	"fmt"
	"sort"
	"strings"
)

type storage interface {
	GaugeInsert(key string, value float64) int
	CounterInsert(key string, value int) int
	GetGauge(key string) (float64, error)
	GetCounter(key string) (int, error)
	GetGaugeMap() map[string]float64
	GetCounterMap() map[string]int
}

type Service struct {
	store *storage
}

func NewService(inst storage) *Service {
	return &Service{store: &inst}
}

func (s *Service) GetGauge(key string) (float64, error) {
	key = strings.ToLower(key)
	return (*s.store).GetGauge(key)
}

func (s *Service) GetCounter(key string) (int, error) {
	key = strings.ToLower(key)
	return (*s.store).GetCounter(key)
}

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

func (s *Service) GaugeInsert(key string, value float64) int {
	key = strings.ToLower(key)
	return (*s.store).GaugeInsert(key, value)
}

func (s *Service) CounterInsert(key string, value int) int {
	key = strings.ToLower(key)
	return (*s.store).CounterInsert(key, value)
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
