package db

import (
	"context"
	"fmt"
	"log"
	"time"

	t "github.com/SourishBeast7/Glooo/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type Store struct {
	userColl     *mongo.Collection
	chatsColl    *mongo.Collection
	messagesColl *mongo.Collection
}

type MyError struct {
	Code    int
	Message string
}

func (e *MyError) Error() string {
	return fmt.Sprintf("Code %d: %s", e.Code, e.Message)
}

func UserExistsError() error {
	return &MyError{
		Code:    402,
		Message: "User Already Exists",
	}
}

func genContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func ConnectMongo() *Store {
	mongoURI := "mongodb://localhost:27017/"
	if mongoURI == "" {
		log.Fatal("❌ MONGO_URI not set in environment")
	}

	clientOptions := options.Client().ApplyURI(mongoURI)

	ctx, cancel := genContext()
	defer cancel()

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Printf("❌ Failed to connect to MongoDB: %s", err.Error())
	}

	// Verify the connection
	if err := client.Ping(ctx, nil); err != nil {
		log.Printf("❌ MongoDB ping failed: %s", err.Error())
	}

	log.Println("✅ Connected to MongoDB")
	return &Store{
		userColl:     client.Database("real").Collection("users"),
		chatsColl:    client.Database("real").Collection("chats"),
		messagesColl: client.Database("real").Collection("messages"),
	}
}

func (s *Store) AddUser(user *t.User) (map[string]any, error) {
	ctx, cancel := genContext()
	errmap := map[string]any{
		"message": "An Error Occured",
	}
	defer cancel()
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return errmap, err
	}
	user.Password = string(hash)
	user.Chats = []t.Chats{}
	now := time.Now()
	user.CreatedAt = now.Format("2006-01-02 15:04:05")
	_, e := s.userColl.InsertOne(ctx, user)
	if e != nil {
		return errmap, e
	}
	log.Println("User Added SuccessFUlly")
	return map[string]any{
		"message": "Account Created Successfully",
	}, nil
}

func (s *Store) UserAlreadyExists(email string) bool {
	ctx, cancel := genContext()
	defer cancel()
	filter := bson.M(map[string]any{"email": email})
	res := s.userColl.FindOne(ctx, filter)
	var user any
	res.Decode(&user)
	return (user != nil)
}

func (s *Store) FindUser(email string, password string) (*t.User, error) {
	ctx, cancel := genContext()
	defer cancel()
	filter := bson.M(map[string]any{"email": email})
	res := s.userColl.FindOne(ctx, filter)
	user := new(t.User)
	if err := res.Decode(user); err != nil {
		return nil, err
	}
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Store) CreateChat(userIds ...string) bool {
	ctx, cancel := genContext()
	defer cancel()
	chat := new(t.Chats)
	chat.Participants = userIds
	chat.Messages = make([]t.Messages, 0)
	_, err := s.chatsColl.InsertOne(ctx, chat)
	if err != nil {
		log.Println(err.Error())
		return false
	}
	return true
}
func (s *Store) AddMessages(message t.Messages) bool {
	ctx, cancel := genContext()
	defer cancel()
	_, err := s.chatsColl.InsertOne(ctx, message)
	if err != nil {
		log.Println(err.Error())
		return false
	}
	return true
}
