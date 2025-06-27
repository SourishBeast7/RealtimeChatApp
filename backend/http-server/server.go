package httpserver

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/SourishBeast7/Glooo/db"
	m "github.com/SourishBeast7/Glooo/http-server/middleware"
	t "github.com/SourishBeast7/Glooo/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/cors"
)

type Server struct {
	listenAddr string
	conn       map[*websocket.Conn]bool
	mutex      sync.RWMutex
	store      *db.Store
}

type handlerFunc func(w http.ResponseWriter, r *http.Request) error

type Response map[string]any

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func NewServer(addr string) *Server {
	return &Server{
		listenAddr: addr,
		conn:       make(map[*websocket.Conn]bool),
	}
}

func (s *Server) NewStore() {
	s.store = db.ConnectMongo()
}

func makeHttpHandler(f handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			log.Println(err.Error())
		}
	}
}

func WriteJson(w http.ResponseWriter, status int, v map[string]any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func uploadFilesToCdn(file io.Reader, email string, filename string) (string, error) {
	var (
		bunnyStorageZone = os.Getenv("BUNNYCDNSZONE")
		bunnyStorageKey  = os.Getenv("BUNNYCDNPASS")
		bunnyStorageHost = os.Getenv("BUNNYCDNHOST")
	)
	uploadURL := fmt.Sprintf("https://%s/%s/profilepictures/%s/%s", bunnyStorageHost, bunnyStorageZone, email, filename)

	req, err := http.NewRequest(http.MethodPut, uploadURL, file)
	if err != nil {
		return "", err
	}
	req.Header.Set("AccessKey", bunnyStorageKey)
	req.Header.Set("Content-Type", "application/octet-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed: %s", string(body))
	}
	return uploadURL, nil
}

func GenerateJWT(user *t.User) (string, error) {
	claims := jwt.MapClaims{
		"email":     user.Email,
		"name":      user.Name,
		"pfp":       user.Pfp,
		"createdAt": user.CreatedAt,
	}
	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

func (s *Server) HandleRoutes() {
	router := mux.NewRouter()
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http:localhost:5173/*", "*"},
		AllowCredentials: true,
		Debug:            true,
	})

	handler := c.Handler(router)
	s.NewStore()
	s.handleAuthRoutes(router.PathPrefix("/auth").Subrouter())
	s.handleChatRoutes(router.PathPrefix("/chat").Subrouter())
	s.handleTestingRoutes(router.PathPrefix("/test").Subrouter())
	router.HandleFunc("/", makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {
		return WriteJson(w, http.StatusOK, Response{
			"message": "Welcome",
		})
	}))
	http.ListenAndServe(s.listenAddr, handler)
}

func (s *Server) handleAuthRoutes(router *mux.Router) {

	router.HandleFunc("/signup", makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {
		user := new(t.User)
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			return err
		}
		user.Name = r.FormValue("name")
		user.Email = r.FormValue("email")
		user.Password = r.FormValue("password")
		file, header, err := r.FormFile("pfp")
		if err != nil {
			return err
		}
		if s.store.UserAlreadyExists(user.Email) {
			return WriteJson(w, http.StatusNotAcceptable, Response{
				"message": "User Already Exists",
			})
		}
		defer file.Close()
		os.MkdirAll("uploads/"+user.Email, os.ModePerm)
		out, err := os.Create("uploads/" + user.Email + "/" + header.Filename)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, file)
		if err != nil {
			return err
		}
		pfp, err := uploadFilesToCdn(file, user.Email, header.Filename)
		if err != nil {
			return err
		}
		user.Pfp = pfp
		res, err := s.store.AddUser(user)
		if err != nil {
			WriteJson(w, http.StatusNotAcceptable, res)
			return err
		}
		return WriteJson(w, http.StatusOK, res)
	})).Methods(http.MethodPost)

	router.HandleFunc("/login", makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {
		u := new(t.TempUser)
		if err := json.NewDecoder(r.Body).Decode(u); err != nil {
			return WriteJson(w, http.StatusNotAcceptable, Response{
				"message": u,
			})
		}
		user, err := s.store.FindUser(u.Email, u.Password)
		if err != nil {
			return err
		}
		token, err := GenerateJWT(user)
		if err != nil {
			WriteJson(w, http.StatusNotAcceptable, Response{
				"success": false,
			})
			return err
		}
		finalToken := fmt.Sprintf("Bearer %s", token)
		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    finalToken,
			HttpOnly: true,
			Path:     "/",
			SameSite: http.SameSiteLaxMode,
			Secure:   false, // Set to true in production with HTTPS
		})
		return WriteJson(w, http.StatusOK, Response{
			"success": true,
		})
	})).Methods(http.MethodPost)
}

//WebSocket - Websocket routes

func (s *Server) handleChatRoutes(router *mux.Router) {
	router.HandleFunc("/", makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {
		return WriteJson(w, http.StatusOK, Response{
			"message": "Chatroom",
		})
	})).Methods("GET")
	router.HandleFunc("/room", makeHttpHandler(s.wsConnHandler))
}

func (s *Server) wsConnHandler(w http.ResponseWriter, r *http.Request) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err.Error())
		return err
	}
	s.mutex.Lock()
	s.conn[conn] = true
	s.mutex.Unlock()

	go s.readLoop(conn)
	return nil
}

func (s *Server) readLoop(con *websocket.Conn) error {
	defer func() {
		s.mutex.Lock()
		delete(s.conn, con)
		s.mutex.Unlock()
		con.Close()
	}()
	for {
		messageType, msg, err := con.ReadMessage()
		s.broadcast(con, websocket.TextMessage, []byte(fmt.Sprintf("Hello User and Your Message : %s", string(msg))))
		if err != nil {
			return err
		}
		log.Println(con.RemoteAddr().String(), messageType, string(msg))
	}
}

func (s *Server) broadcast(sender *websocket.Conn, messageType int, data []byte) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for con, b := range s.conn {
		if !b || con == sender {
			continue
		}
		if err := con.WriteMessage(messageType, data); err != nil {
			return err
		}
	}
	return nil
}

//Testing Routes Start

func (s *Server) handleTestingRoutes(router *mux.Router) {
	// router.Use(middleware.AuthMiddleware)
	router.HandleFunc("/", makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {
		return WriteJson(w, http.StatusOK, Response{
			"info": "test route",
		})
	})).Methods(http.MethodGet)
	router.HandleFunc("/t1", m.TestMiddleWare(makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {

		return WriteJson(w, http.StatusOK, Response{
			"message": "Destination Reached",
		})
	})))
}

// Testing routes End
