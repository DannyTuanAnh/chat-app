include .env
export

AUTH_CONN_STRING = "postgresql://$(DB_USER):$(AUTH_DB_PASSWORD)@$(DB_HOST):$(AUTH_DB_PORT)/$(AUTH_DB_NAME)?sslmode=$(DB_SSLMODE)"
USER_CONN_STRING = "postgresql://$(DB_USER):$(USER_DB_PASSWORD)@$(DB_HOST):$(USER_DB_PORT)/$(USER_DB_NAME)?sslmode=$(DB_SSLMODE)"
FRIEND_CONN_STRING = "postgresql://$(DB_USER):$(FRIEND_DB_PASSWORD)@$(DB_HOST):$(FRIEND_DB_PORT)/$(FRIEND_DB_NAME)?sslmode=$(DB_SSLMODE)"
CHAT_CONN_STRING = "postgresql://$(DB_USER):$(CHAT_DB_PASSWORD)@$(DB_HOST):$(CHAT_DB_PORT)/$(CHAT_DB_NAME)?sslmode=$(DB_SSLMODE)"

AUTH_MIGRATION_DIRS = ./internal/db/migrations/auth
USER_MIGRATION_DIRS = ./internal/db/migrations/user
FRIEND_MIGRATION_DIRS = ./internal/db/migrations/friend
CHAT_MIGRATION_DIRS = ./internal/db/migrations/chat

# import database
import-db:
	docker exec -i postgres-db psql -U root -d master-golang < ./backupdb-master-master-golang.sql

# export database
export-db:
	docker exec -i postgres-db-chat-app pg_dump -U root -d chat-app > ./backupdb-master-master-golang.sql

# create new migration file (Example: make migrate-create name=profiles)
migrate-create-auth-db:
	migrate create -ext sql -dir $(AUTH_MIGRATION_DIRS) -seq $(name)
migrate-create-user-db:
	migrate create -ext sql -dir $(USER_MIGRATION_DIRS) -seq $(name) 
migrate-create-friend-db:
	migrate create -ext sql -dir $(FRIEND_MIGRATION_DIRS) -seq $(name)
migrate-create-chat-db:
	migrate create -ext sql -dir $(CHAT_MIGRATION_DIRS) -seq $(name)


# run all up migrations (Example: make migrate-up)
migrate-up-auth-db: 
	migrate -path $(AUTH_MIGRATION_DIRS) -database "$(AUTH_CONN_STRING)" up
migrate-up-user-db: 
	migrate -path $(USER_MIGRATION_DIRS) -database "$(USER_CONN_STRING)" up
migrate-up-friend-db: 
	migrate -path $(FRIEND_MIGRATION_DIRS) -database "$(FRIEND_CONN_STRING)" up
migrate-up-chat-db: 
	migrate -path $(CHAT_MIGRATION_DIRS) -database "$(CHAT_CONN_STRING)" up

# rollback the last migration (Example: make migrate-down)
migrate-down-auth-db:
	migrate -path $(AUTH_MIGRATION_DIRS) -database "$(AUTH_CONN_STRING)" down 1
migrate-down-user-db:
	migrate -path $(USER_MIGRATION_DIRS) -database "$(USER_CONN_STRING)" down 1
migrate-down-friend-db:
	migrate -path $(FRIEND_MIGRATION_DIRS) -database "$(FRIEND_CONN_STRING)" down 1
migrate-down-chat-db:
	migrate -path $(CHAT_MIGRATION_DIRS) -database "$(CHAT_CONN_STRING)" down 1

# rollback n migration (Example: make migrate-down n=2)
migrate-down-n-auth-db:
	migrate -path $(AUTH_MIGRATION_DIRS) -database "$(AUTH_CONN_STRING)" down $(n)
migrate-down-n-user-db:
	migrate -path $(USER_MIGRATION_DIRS) -database "$(USER_CONN_STRING)" down $(n)
migrate-down-n-friend-db:
	migrate -path $(FRIEND_MIGRATION_DIRS) -database "$(FRIEND_CONN_STRING)" down $(n)
migrate-down-n-chat-db:
	migrate -path $(CHAT_MIGRATION_DIRS) -database "$(CHAT_CONN_STRING)" down $(n)

# force migration version (use with caution, Example: make migrate-force version=2) 
migrate-force-auth-db:
	migrate -path $(AUTH_MIGRATION_DIRS) -database "$(AUTH_CONN_STRING)" force $(version)
migrate-force-user-db:
	migrate -path $(USER_MIGRATION_DIRS) -database "$(USER_CONN_STRING)" force $(version)
migrate-force-friend-db:
	migrate -path $(FRIEND_MIGRATION_DIRS) -database "$(FRIEND_CONN_STRING)" force $(version)
migrate-force-chat-db:
	migrate -path $(CHAT_MIGRATION_DIRS) -database "$(CHAT_CONN_STRING)" force $(version)

# drop everything (include schema migration)
migrate-drop:
	migrate -path $(MIGRATION_DIRS) -database "$(CONN_STRING)" drop 

# apply specific migration version (Example: make migrate-goto version=2)
migrate-goto:
	migrate -path $(MIGRATION_DIRS) -database "$(CONN_STRING)" goto $(version)

# Generate Go code from SQL queries using sqlc
sqlc:
	sqlc generate

buf:
	buf dep update

# Makefile for generating gRPC code from .proto files
# Example: make gen-proto path=calculator/calculatorpb/calculator.proto
# path is the path to your .proto file
gen-proto:
	protoc $(path) --go_out=. --go-grpc_out=.

run-api:
	go run cmd/api/main.go

run-auth:
	go run cmd/auth/main.go

run-user:
	go run cmd/user/main.go

run-friend:
	go run cmd/friend/main.go

.PHONY: import-db export-db migrate-create migrate-up migrate-down migrate-force migrate-drop migrate-goto sqlc gen-proto run-api run-auth run-user buf
