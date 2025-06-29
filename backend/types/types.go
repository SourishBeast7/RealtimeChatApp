package types

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Chats struct {
	Group        bool
	Participants []primitive.ObjectID `json:"participants"`
	Messages     []primitive.ObjectID `json:"messages"`
}

type Message struct {
	Data        string             `json:"data"`
	ArrivalTime string             `json:"arrivalTime"`
	Owner       primitive.ObjectID `json:"owner"`
}

type TempUser struct {
	Email    string
	Password string
}

type MongoUser struct {
	ID        primitive.ObjectID   `bson:"_id,omitempty" json:"id,omitempty"`
	Email     string               `json:"email"`
	Name      string               `json:"name"`
	Password  string               `json:"-"`
	Pfp       string               `json:"pfp"`
	CreatedAt string               `json:"createdAt"`
	Chats     []primitive.ObjectID `json:"chats"`
}
