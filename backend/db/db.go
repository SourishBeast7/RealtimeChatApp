package db

import (
	"context"
	"fmt"
	"log"
	"time"

	t "github.com/SourishBeast7/Glooo/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

func logError(err error) {
	log.Printf("%+v", err)
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

// Operations on User Collections

func (s *Store) AddUser(user *t.MongoUser) (map[string]any, error) {
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
	user.Chats = []primitive.ObjectID{}
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

func (s *Store) FindUserByEmail(email string) (*t.MongoUser, error) {
	ctx, cancel := genContext()
	defer cancel()
	filter := bson.M{"email": email}
	res := s.userColl.FindOne(ctx, filter)
	user := new(t.MongoUser)
	if err := res.Decode(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Store) FindUserById(id primitive.ObjectID) (*t.MongoUser, error) {
	ctx, cancel := genContext()
	defer cancel()
	filter := bson.M{"_id": id}
	res := s.userColl.FindOne(ctx, filter)
	user := new(t.MongoUser)
	if err := res.Decode(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Store) UserAlreadyExists(email string) bool {
	ctx, cancel := genContext()
	defer cancel()
	filter := bson.M(map[string]any{"email": email})
	res := s.userColl.FindOne(ctx, filter)
	user := new(t.MongoUser)
	res.Decode(user)
	return (user != nil)
}

func (s *Store) AuthenticateUser(email string, password string) (*t.MongoUser, error) {
	user, err := s.FindUserByEmail(email)
	if err != nil {
		return nil, err
	}
	e := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if e != nil {
		return nil, e
	}
	return user, nil
}

func (s *Store) UpdateUserDetails(id primitive.ObjectID, field string, value any) error {
	ctx, cancel := genContext()
	defer cancel()
	filter := bson.M{
		"_id": id,
	}
	data := bson.M{
		"$set": bson.M{
			field: value,
		},
	}
	res := s.userColl.FindOneAndUpdate(ctx, filter, data)
	if err := res.Err(); err != nil {
		return err
	}
	newUser := new(t.MongoUser)
	if err := res.Decode(newUser); err != nil {
		return err
	}
	return nil
}

// Operations on User Collections - end
// Operations on Chats Collections

func (s *Store) CreateChat(userIds ...primitive.ObjectID) (primitive.ObjectID, bool) {
	ctx, cancel := genContext()
	defer cancel()
	chat := new(t.Chats)
	chat.Participants = userIds
	n := primitive.NilObjectID
	if len(userIds) == 2 {
		to, err := s.FindUserById(userIds[1])
		if err != nil {
			log.Printf("Create Chat Error : %v", err)
			return n, false
		}
		chat.Name = to.Name
		chat.Group = false
	}
	seen := make(map[primitive.ObjectID]bool)
	for _, us := range chat.Participants {
		if seen[us] {
			return n, false
		}
		seen[us] = true
	}
	chat.Messages = make([]primitive.ObjectID, 0)
	res, err := s.chatsColl.InsertOne(ctx, chat)
	chatId, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return n, false
	}
	if err != nil {
		log.Println(err.Error())
		return n, false
	}
	for _, id := range userIds {
		user, err := s.FindUserById(id)
		if err != nil {
			log.Printf("%s", err.Error())
			return n, false
		}
		user.Chats = append(user.Chats, chatId)
		err = s.UpdateUserDetails(id, "chats", user.Chats)
		if err != nil {
			logError(err)
			log.Println(err.Error())
			return n, false
		}
	}
	return chatId, true
}

func (s *Store) GetChatsByUserId(userid string) ([]t.Chats, error) {
	ctx, cancel := genContext()
	defer cancel()
	objid, err := primitive.ObjectIDFromHex(userid)
	if err != nil {
		return nil, err
	}
	user, err := s.FindUserById(objid)
	if err != nil {
		return nil, err
	}
	filter := bson.M{
		"participants": user.ID,
	}
	c, er := s.chatsColl.Find(ctx, filter)
	if er != nil {
		return nil, er
	}
	chats := make([]t.Chats, 0)
	e := c.All(ctx, &chats)
	if e != nil {
		return nil, er
	}
	return chats, nil
}

func (s *Store) FindChatById(id primitive.ObjectID) (*t.Chats, error) {
	ctx, cancel := genContext()
	defer cancel()

	filter := bson.M{"_id": id}
	res := s.chatsColl.FindOne(ctx, filter)

	if err := res.Err(); err != nil {
		return nil, err
	}

	chat := new(t.Chats)
	if err := res.Decode(chat); err != nil {
		return nil, err
	}

	return chat, nil
}

func (s *Store) FindChatByParticipants(ids ...primitive.ObjectID) *t.Chats {
	ctx, cancel := genContext()
	defer cancel()
	filter := bson.M{
		"$in": bson.M{
			"participants": ids,
		},
	}
	res := s.chatsColl.FindOne(ctx, filter)
	if err := res.Err(); err != nil {
		log.Println(err.Error())
		return nil
	}
	chat := new(t.Chats)
	if err := res.Decode(chat); err != nil {
		log.Println(err.Error())
		return nil
	}
	return chat
}

// Operations on Chats Collections - end

// Operations on Message Collection

// func (s *Store) AddMessages(message *t.Message, chatid primitive.ObjectID) bool {
// 	ctx, cancel := genContext()
// 	defer cancel()
// 	now := time.Now()
// 	message.ArrivalTime = now.Format("2006-01-02 15:04:05")
// 	session, err := s.messagesColl.Database().Client().StartSession()
// 	if err != nil {
// 		log.Println(err.Error())
// 		return false
// 	}
// }

func (s *Store) FindMessagesById(id primitive.ObjectID) *t.Message {
	ctx, cancel := genContext()
	defer cancel()
	filter := bson.M{
		"_id": id,
	}
	res := s.messagesColl.FindOne(ctx, filter)
	if err := res.Err(); err != nil {
		return nil
	}
	message := new(t.Message)
	res.Decode(message)
	return message
}

func (s *Store) FindMessagesByChatId(chatid string) ([]t.Message, bool) {
	ctx, cancel := genContext()
	defer cancel()
	filter := bson.M{
		"chatid": chatid,
	}
	res, err := s.messagesColl.Find(ctx, filter)
	if err != nil {
		log.Printf("FindMessagesByChatId Error :%v", err)
		return nil, false
	}
	messages := make([]t.Message, 0)
	err = res.All(ctx, &messages)
	if err != nil {
		return nil, false
	}
	return messages, true
}

func (s *Store) InsertMessageInChat(chatId, msgId primitive.ObjectID) bool {
	ctx, cancel := genContext()
	defer cancel()
	chat, err := s.FindChatById(chatId)
	if err != nil {
		return false
	}
	chat.Messages = append(chat.Messages, msgId)
	data := bson.M{
		"$push": bson.M{
			"messages": msgId,
		},
	}
	_, err = s.chatsColl.UpdateByID(ctx, chatId, data)
	if err != nil {
		log.Printf("%v", err)
		return false
	}
	return true
}
