package middleware

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("token")
		if err != nil {
			log.Println(err.Error())
			http.Error(w, "Invalid Token String", http.StatusUnauthorized)
		}

		tokenStr := cookie.Value

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if m, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unrecognized signing method : %v", m)
			}
			return os.Getenv("JWT_SECRET"), nil
		})
		if err != nil || !token.Valid {
			log.Printf("%+v", token.Claims)
			http.Error(w, "Unauthorized - Invalid Token", http.StatusUnauthorized)
			return
		}
		log.Printf("%+v", token.Claims)
		next.ServeHTTP(w, r)
	})
}

func TestMiddleWare(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("token")
		if err != nil {
			log.Println(err.Error())
			http.Error(w, "Invalid Token String", http.StatusUnauthorized)
			return
		}

		tokenStr := strings.Split(cookie.Value, " ")[1]
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			if m, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unrecognized signing method : %v", m)
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if err != nil || !token.Valid {
			log.Printf("%+v", err.Error())
			http.Error(w, "Unauthorized - Invalid Token", http.StatusUnauthorized)
			return
		}
		f(w, r)
	}
}
