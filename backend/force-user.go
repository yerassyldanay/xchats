package main

import (
	"context"
	"fmt"


	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
)

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://postgres:postgres@localhost:5434/xchats?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	hash, _ := httpapi.HashPassword("password123")
	fmt.Println("Hash:", hash)

	orgID := uuid.New()
	_, err = pool.Exec(ctx, "INSERT INTO xchats.organizations (id, name) VALUES ($1, 'Test Org') ON CONFLICT DO NOTHING", orgID)
	if err != nil {
		panic(err)
	}

	_, err = pool.Exec(ctx, "INSERT INTO xchats.users (email, password_hash, display_name) VALUES ($1, $2, 'Admin') ON CONFLICT (email) DO UPDATE SET password_hash = $2", "admin@example.com", hash)
	if err != nil {
		panic(err)
	}
	
	fmt.Println("User inserted: admin@example.com / password123")
}
