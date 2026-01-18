// Package netutil предоставляет утилиты для работы с сетью.
package netutil

import (
	"fmt"
	"net"
)

// GetOutboundIP возвращает предпочтительный исходящий IP-адрес хоста.
//
// Метод создаёт UDP-соединение к внешнему адресу (без реальной отправки данных)
// и извлекает локальный IP-адрес, который был бы использован для этого соединения.
// Это позволяет определить "основной" IP-адрес машины в сети.
func GetOutboundIP() (net.IP, error) {
	// Создаём UDP-соединение к внешнему адресу (Google DNS)
	// Данные не отправляются, соединение не устанавливается (UDP connectionless)
	// Адрес назначения может не существовать - важен только выбор локального интерфейса
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, fmt.Errorf("failed to determine outbound IP: %w", err)
	}
	defer conn.Close()

	// Извлекаем локальный адрес соединения
	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, fmt.Errorf("unexpected address type: %T", conn.LocalAddr())
	}

	return localAddr.IP, nil
}

// GetOutboundIPString возвращает предпочтительный исходящий IP-адрес как строку.
func GetOutboundIPString() (string, error) {
	ip, err := GetOutboundIP()
	if err != nil {
		return "", err
	}
	return ip.String(), nil
}

// GetLocalIP возвращает первый не-loopback IPv4 адрес хоста.
// Альтернативный метод через перебор сетевых интерфейсов.
func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("failed to get interface addresses: %w", err)
	}

	for _, addr := range addrs {
		// Проверяем, что это IP-сеть и не loopback
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			// Берём только IPv4 адреса
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no suitable local IP address found")
}
