package types

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Chats struct {
	ID           primitive.ObjectID   `bson:"_id,omitempty" json:"id,omitempty"`
	Name         string               `json:"name"`
	Group        bool                 `json:"group"`
	Participants []primitive.ObjectID `json:"participants"`
	Messages     []primitive.ObjectID `json:"messages"`
}

type Message struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Data        string             `json:"data"`
	ArrivalTime string             `json:"arrivalTime"`
	From        primitive.ObjectID `json:"from"`
	ChatId      primitive.ObjectID `json:"chatid"`
	To          primitive.ObjectID `json:"to"`
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
