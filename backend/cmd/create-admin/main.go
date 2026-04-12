// create-admin — bootstrap the admin user (or reset their password).
// Usage:
//
//	go run ./cmd/create-admin -username=admin -password=secret
//	ADMIN_USERNAME=admin ADMIN_PASSWORD=secret go run ./cmd/create-admin
package main

import (
	"errors"
	"flag"
	"log"
	"os"

	"github.com/mac/claudemote/backend/internal/config"
	"github.com/mac/claudemote/backend/internal/database"
	"github.com/mac/claudemote/backend/internal/model"
	"github.com/mac/claudemote/backend/internal/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
	usernameFlag := flag.String("username", os.Getenv("ADMIN_USERNAME"), "admin username")
	passwordFlag := flag.String("password", os.Getenv("ADMIN_PASSWORD"), "admin plaintext password")
	flag.Parse()

	if *usernameFlag == "" || *passwordFlag == "" {
		log.Fatal("missing -username or -password (or ADMIN_USERNAME / ADMIN_PASSWORD env vars)")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("config: ", err)
	}

	db, err := database.Connect(cfg.DBPath)
	if err != nil {
		log.Fatal("connect: ", err)
	}

	if err := database.RunMigrations(db); err != nil {
		log.Fatal("migrations: ", err)
	}

	repo := repository.NewUserRepository(db)

	hash, err := bcrypt.GenerateFromPassword([]byte(*passwordFlag), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal("hash password: ", err)
	}

	existing, err := repo.FindByUsername(*usernameFlag)
	if err == nil {
		// User exists — reset password.
		existing.PasswordHash = string(hash)
		if err := db.Save(existing).Error; err != nil {
			log.Fatal("update password: ", err)
		}
		log.Printf("password reset for user: %s", *usernameFlag)
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Fatal("lookup user: ", err)
	}

	// New user — insert.
	user := &model.User{
		Username:     *usernameFlag,
		PasswordHash: string(hash),
	}
	if err := repo.Create(user); err != nil {
		log.Fatal("create user: ", err)
	}
	log.Printf("admin user created: %s", *usernameFlag)
}
