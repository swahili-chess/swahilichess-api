// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0

package db

import (
	"context"

	"github.com/google/uuid"
)

type Querier interface {
	CreateToken(ctx context.Context, arg CreateTokenParams) error
	CreateUser(ctx context.Context, arg CreateUserParams) (CreateUserRow, error)
	DeleteToken(ctx context.Context, arg DeleteTokenParams) error
	DeleteUserById(ctx context.Context, id uuid.UUID) error
	GetActiveTgBotUsers(ctx context.Context) ([]int64, error)
	GetLichessTeamMembers(ctx context.Context) ([]string, error)
	GetUserById(ctx context.Context, id uuid.UUID) (GetUserByIdRow, error)
	GetUserByToken(ctx context.Context, arg GetUserByTokenParams) (GetUserByTokenRow, error)
	GetUserByUsername(ctx context.Context, username string) (GetUserByUsernameRow, error)
	GetUserByUsernameOrPhone(ctx context.Context, arg GetUserByUsernameOrPhoneParams) (User, error)
	GetUserForResetOrActivation(ctx context.Context, arg GetUserForResetOrActivationParams) (GetUserForResetOrActivationRow, error)
	InsertLichessTeamMember(ctx context.Context, arg InsertLichessTeamMemberParams) error
	InsertTgBotUsers(ctx context.Context, arg InsertTgBotUsersParams) error
	UpdateTgBotUsers(ctx context.Context, arg UpdateTgBotUsersParams) error
	UpdateUserById(ctx context.Context, arg UpdateUserByIdParams) error
}

var _ Querier = (*Queries)(nil)
