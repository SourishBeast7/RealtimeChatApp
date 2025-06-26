package types

import "time"

type User struct {
	Email     string  `json:"email"`
	Name      string  `json:"name"`
	Password  string  `json:"password"`
	Pfp       string  `json:"pfp"`
	CreatedAt string  `json:"createdAt"`
	Chats     []Chats `json:"chats"`
}

type Chats struct {
	Participants []string   `json:"participants"`
	Messages     []Messages `json:"messages"`
}

type Messages struct {
	Data        string    `json:"data"`
	ArrivalTime time.Time `json:"arrivalTime"`
	Owner       string    `json:"owner"`
}

type TempUser struct {
	Email    string
	Password string
}
