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
	"github.com/SourishBeast7/Glooo/types"
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
		bunnyStorageZone = "sourishbeast7"
		bunnyStorageKey  = "b3333f3d-cf54-46be-b8187eb7dd44-ddd0-499f"
		bunnyStorageHost = "sg.storage.bunnycdn.com"
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
		user := new(types.User)
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
			return err
		}
		return WriteJson(w, http.StatusOK, res)
	})).Methods(http.MethodPost)

	router.HandleFunc("/login", makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {
		return WriteJson(w, http.StatusOK, Response{
			"message": "Auth Router",
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
	// router.HandleFunc("/{id}", makeHttpHandler(s.openChat)).Methods("GET")
	router.HandleFunc("/room", makeHttpHandler(s.wsConnHandler))
}

//Testing Routes Start

func (s *Server) handleTestingRoutes(router *mux.Router) {
	router.HandleFunc("/upload", makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {
		return WriteJson(w, http.StatusOK, Response{
			"info": "test route",
		})
	})).Methods(http.MethodGet)
}

// Testing routes End
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
