package main

import (
	"log"
	"os"
	"github.com/gauravnetes/go-cask-db/internal/engine"
	"github.com/gauravnetes/go-cask-db/internal/network"
)


// func main() {
// 	err := os.MkdirAll("./data", 0755)
// 	if err != nil {
// 		log.Fatalf("Failed to create data directory: %v", err)
// 	}

// 	db, err := engine.NewDB("./data")
// 	if err != nil {
// 		log.Fatalf("Failed to intitialize database: %v", err)
// 	}

// 	fmt.Println("Database initialized successfully")
// 	fmt.Println("Injecting 3000 records...")

// 	for i := 1; i <= 3000; i++ {
// 		key := fmt.Sprintf("user_%d", i)
// 		val := []byte(fmt.Sprintf("Hello, I'm user number %d", i))

// 		err := db.Put(key, val)
// 		if err != nil {
// 			log.Fatalf("Failed to write key %s: %v", key, err)
// 		}
// 	}
	
// 	fmt.Println("Injection complete!")
// 	fmt.Println("Check out ./data folder to see the SSTables")


// }



func main() {
	// 1. Ensure data directory exists
	err := os.MkdirAll("./data", 0755)
	if err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// 2. Initialize the Database Engine
	db, err := engine.NewDB("./data")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 3. Initialize and Start the TCP Server on port 8080
	server := network.NewServer(db, ":8080")
	err = server.Start()
	if err != nil {
		log.Fatalf("Server crashed: %v", err)
	}
}