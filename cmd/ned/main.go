package main

import "nedgentic/internal/frontend"

var BuildTime = "dev"

func main() {
	frontend.Execute(BuildTime)
}
