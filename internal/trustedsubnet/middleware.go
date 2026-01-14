// Package trustedsubnet предоставляет middleware для проверки IP-адреса агента
// на принадлежность к доверенной подсети (CIDR).
package trustedsubnet

import (
	"fmt"
	"net"
	"net/http"
)

// TrustedSubnetMiddleware создаёт middleware для проверки IP-адреса агента.
//
// Логика работы:
//   - Если trustedSubnet пустой - все запросы пропускаются без ограничений
//   - IP-адрес агента берётся ТОЛЬКО из заголовка X-Real-IP
//   - Если заголовок отсутствует или IP не входит в подсеть - возвращается 403 Forbidden
//
// Параметры:
//   - trustedSubnet: строка в CIDR нотации (например, "192.168.1.0/24")
func TrustedSubnetMiddleware(trustedSubnet string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// При пустом значении trusted_subnet - пропускаем все запросы без ограничений
			if trustedSubnet == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Парсим CIDR подсеть (делаем один раз при старте в продакшене,
			// здесь для простоты - при каждом запросе)
			_, ipNet, err := net.ParseCIDR(trustedSubnet)
			if err != nil {
				// Невалидный CIDR в конфигурации - серверная ошибка
				http.Error(w, "Internal server error: invalid trusted subnet configuration", http.StatusInternalServerError)
				return
			}

			// Получаем IP агента СТРОГО из заголовка X-Real-IP
			realIPHeader := r.Header.Get("X-Real-IP")

			// Если заголовок X-Real-IP отсутствует - запрещаем доступ
			if realIPHeader == "" {
				http.Error(w, "Forbidden: X-Real-IP header is required", http.StatusForbidden)
				return
			}

			// Парсим IP-адрес из заголовка
			clientIP := net.ParseIP(realIPHeader)
			if clientIP == nil {
				http.Error(w, fmt.Sprintf("Forbidden: invalid IP address in X-Real-IP header: %s", realIPHeader), http.StatusForbidden)
				return
			}

			// Проверяем принадлежность IP агента к доверенной подсети
			if !ipNet.Contains(clientIP) {
				http.Error(w, fmt.Sprintf("Forbidden: IP %s is not in trusted subnet", clientIP), http.StatusForbidden)
				return
			}

			// IP в доверенной подсети - передаём запрос дальше
			next.ServeHTTP(w, r)
		})
	}
}
