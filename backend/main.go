package main

import (
	g "github.com/SourishBeast7/Glooo/http-server"
)

func main() {
	server := g.NewServer(":3000")
	server.HandleRoutes()
}
