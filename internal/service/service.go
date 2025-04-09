package service

import (
	"fmt"
	"sync"
)

type Storage interface {
	GaugeInsert(key string, value float64) int
	CounterInsert(key string, value int) int
	GetGauge(key string) (float64, error)
	GetCounter(key string) (int, error)
	GetGaugeMap() map[string]float64
	GetCounterMap() map[string]int
}

type Service struct {
	mu    sync.RWMutex
	store Storage
}

func NewService(inst Storage) *Service {
	return &Service{store: inst}
}

func (s *Service) GetGauge(key string) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store.GetGauge(key)
}

func (s *Service) GetCounter(key string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store.GetCounter(key)
}

func (s *Service) GetAllMetrics() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]string)
	for key, gauge := range s.store.GetGaugeMap() {
		result[key] = fmt.Sprintf("%v", gauge)
	}
	for key, counter := range s.store.GetCounterMap() {
		result[key] = fmt.Sprintf("%v", counter)
	}
	return result
}

func (s *Service) GaugeInsert(key string, value float64) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.GaugeInsert(key, value)
}

func (s *Service) CounterInsert(key string, value int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.CounterInsert(key, value)
}

func (s *Service) GetUpdateUrls(host string, port string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	urls := []string{}
	gaugeMap := s.store.GetGaugeMap()
	counterMap := s.store.GetCounterMap()
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
