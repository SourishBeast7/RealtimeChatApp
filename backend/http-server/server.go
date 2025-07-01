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
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	CheckOrigin: func(r *http.Request) bool {
		return true // ✅ Allow all origins temporarily
	},
}

func NewServer(addr string) *Server {
	return &Server{
		listenAddr: addr,
		conn:       make(map[*websocket.Conn]bool),
		store:      db.ConnectMongo(),
	}
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

func GenerateJWT(user *t.MongoUser) (string, error) {
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
		AllowedOrigins:   []string{"http://localhost:5173", "http:localhost:5173/*"},
		AllowCredentials: true,
		Debug:            true,
		AllowedHeaders:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	})

	handler := c.Handler(router)
	s.handleAuthRoutes(router.PathPrefix("/auth").Subrouter())
	s.handleChatRoutes(router.PathPrefix("/chat").Subrouter())
	s.handleApiRoutes(router.PathPrefix("/api").Subrouter())
	s.handleTestingRoutes(router.PathPrefix("/test").Subrouter())

	router.HandleFunc("/", makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {
		return WriteJson(w, http.StatusOK, Response{
			"message": "Welcome",
		})
	}))
	log.Printf("🚀 Server started on http://localhost%s", s.listenAddr)
	http.ListenAndServe(s.listenAddr, handler)
}

func (s *Server) handleAuthRoutes(router *mux.Router) {

	router.HandleFunc("/signup", makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {
		user := new(t.MongoUser)
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
		user, err := s.store.AuthenticateUser(u.Email, u.Password)
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
		id := user.ID.Hex()
		finalToken := fmt.Sprintf("Bearer %s", token)
		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    finalToken,
			HttpOnly: true,
			Path:     "/",
			SameSite: http.SameSiteLaxMode,
			Secure:   false, // Set to true in production with HTTPS
		})
		http.SetCookie(w, &http.Cookie{
			Name:     "UID",
			Value:    string(id),
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

func (s *Server) handleApiRoutes(router *mux.Router) {
	router.HandleFunc("/getchats", m.AuthMiddleWare(makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {
		userId, err := r.Cookie("UID")
		if err != nil {
			WriteJson(w, http.StatusNotAcceptable, Response{
				"err": err,
			})
			return err
		}

		res, err := s.store.GetChatsByUserId(userId.Value)
		if err != nil {
			return err
		}
		return WriteJson(w, http.StatusOK, Response{
			"data": res,
		})
	}))).Methods(http.MethodGet)

	router.HandleFunc("/getmessages", m.AuthMiddleWare(makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {
		chatid, err := r.Cookie("UID")
		if err != nil {
			WriteJson(w, http.StatusNotAcceptable, Response{
				"err": err,
			})
			return err
		}
		messages, ok := s.store.FindMessagesByChatId(chatid.Value)
		return WriteJson(w, http.StatusOK, Response{
			"success":  ok,
			"messages": messages,
		})
	})))

	router.HandleFunc("/chat/create", m.AuthMiddleWare(makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {
		userId, err := r.Cookie("UID")
		if err != nil {
			WriteJson(w, http.StatusNotAcceptable, Response{
				"err": err.Error(),
			})
			return err
		}
		id, err := primitive.ObjectIDFromHex(userId.Value)
		if err != nil {
			return err
		}
		user1, e := s.store.FindUserById(id)
		if e != nil {
			WriteJson(w, http.StatusNotAcceptable, Response{
				"err": e.Error(),
			})
			return e
		}
		data := make(map[string]string, 0)
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			WriteJson(w, http.StatusNotAcceptable, Response{
				"err": err.Error(),
			})
			return err
		}
		user2, err := s.store.FindUserByEmail(data["email"])
		if err != nil {
			WriteJson(w, http.StatusNotAcceptable, Response{
				"err": err.Error(),
			})
			return err
		}
		res, success := s.store.CreateChat(user1.ID, user2.ID)
		if !success {
			return WriteJson(w, http.StatusNotAcceptable, Response{
				"success": success,
			})
		}
		return WriteJson(w, http.StatusOK, Response{
			"id": res,
		})

	}))).Methods(http.MethodPost)
}

//WebSocket - Websocket routes

func (s *Server) handleChatRoutes(router *mux.Router) {
	router.HandleFunc("/{chatid}", m.AuthMiddleWare(makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {
		userId := mux.Vars(r)["chatid"]
		return WriteJson(w, http.StatusOK, Response{
			"chat id": userId,
		})
	}))).Methods("GET")
	// router.HandleFunc("/", m.AuthMiddleWare(makeHttpHandler(s.wsConnHandler))).Methods(http.MethodGet)
}

// func (s *Server) wsConnHandler(w http.ResponseWriter, r *http.Request) error {
// 	log.Println("➡️ Incoming WebSocket request...")
// 	conn, err := upgrader.Upgrade(w, r, nil)
// 	if err != nil {
// 		log.Println("❌ WebSocket upgrade failed:", err)
// 		return err
// 	}

// 	log.Println("✅ WebSocket connection upgraded")
// 	userId, err := r.Cookie("UID")
// 	if err != nil {
// 		return err
// 	}

// 	s.mutex.Lock()
// 	s.conn[conn] = true
// 	s.mutex.Unlock()

// 	go s.readLoop(conn, userId.Value)
// 	return nil
// }

// func (s *Server) readLoop(con *websocket.Conn, userID string) {
// 	defer func() {
// 		s.mutex.Lock()
// 		delete(s.conn, con)
// 		s.mutex.Unlock()
// 		con.Close()
// 	}()
// 	for {
// 		var m map[string]string
// 		err := con.ReadJSON(&m)
// 		if err != nil {
// 			log.Printf("%+v", err)
// 			continue
// 		}
// 		from, err := primitive.ObjectIDFromHex(userID)
// 		if err != nil {
// 			log.Printf("%+v", err)
// 			continue
// 		}
// 		to, err := s.store.FindUserByEmail(m["to"])
// 		if err != nil {
// 			log.Println(err.Error())
// 			continue
// 		}
// 		message := new(t.Message)
// 		message.Data = m["data"]
// 		message.To = to.ID
// 		message.From = from
// 		id, ok := s.store.CreateChat(from, message.To)
// 		if !ok {
// 			log.Println("Chat Creation Failed")
// 			continue
// 		}
// 		message.ChatId = id
// 		message.ArrivalTime = time.Now().Format("2006-01-02 15:04:05")

// 		res := s.store.AddMessages(message, id)
// 		if !res {
// 			log.Println("AddMessages Failed")
// 			return
// 		}
// 		log.Printf("%v", message)
// 	}
// }

// func (s *Server) broadcast(sender *websocket.Conn, messageType int, data []byte) error {
// 	s.mutex.Lock()
// 	defer s.mutex.Unlock()
// 	for con, b := range s.conn {
// 		if !b || con == sender {
// 			continue
// 		}
// 		if err := con.WriteMessage(messageType, data); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

//Testing Routes Start

func (s *Server) handleTestingRoutes(router *mux.Router) {
	router.HandleFunc("/", makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {
		return WriteJson(w, http.StatusOK, Response{
			"info": "test route",
		})
	})).Methods(http.MethodGet)
	router.HandleFunc("/t1", m.AuthMiddleWare(makeHttpHandler(func(w http.ResponseWriter, r *http.Request) error {
		return WriteJson(w, http.StatusOK, Response{
			"message": "Destination Reached",
		})
	})))
}

// Testing routes End
