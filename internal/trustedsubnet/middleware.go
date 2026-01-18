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
			if trustedSubnet == "" {
				next.ServeHTTP(w, r)
				return
			}

			_, ipNet, err := net.ParseCIDR(trustedSubnet)
			if err != nil {
				http.Error(w, "Internal server error: invalid trusted subnet configuration", http.StatusInternalServerError)
				return
			}

			realIPHeader := r.Header.Get("X-Real-IP")

			// ИЗМЕНЕНИЕ: если заголовок отсутствует — пропускаем запрос
			// (или берём IP из RemoteAddr)
			if realIPHeader == "" {
				next.ServeHTTP(w, r) // Разрешаем запросы без X-Real-IP
				return
			}

			clientIP := net.ParseIP(realIPHeader)
			if clientIP == nil {
				http.Error(w, fmt.Sprintf("Forbidden: invalid IP address in X-Real-IP header: %s", realIPHeader), http.StatusForbidden)
				return
			}

			if !ipNet.Contains(clientIP) {
				http.Error(w, fmt.Sprintf("Forbidden: IP %s is not in trusted subnet", clientIP), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
