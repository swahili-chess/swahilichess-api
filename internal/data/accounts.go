package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"backend.chesswahili.com/internal/validator"
)

var (
	ErrDuplicateChesscomUsername = errors.New("given chess.com username already exits")
	ErrDuplicateLichessUsername  = errors.New("given lichess username already exists")
	ErrDuplicatePhonenumber      = errors.New("given phone number already exists")
)

type Account struct {
	AccountID        string    `json:"account_id"`
	UserID           string    `json:"user_id"`
	Firstname        string    `json:"firstname"`
	Lastname         string    `json:"lastname"`
	LichessUsername  string    `json:"lichess_username"`
	ChesscomUsername string    `json:"chesscom_username"`
	PhoneNumber      string    `json:"phone_number"`
	CreatedAt        time.Time `json:"created_at"`
	Photo            byte    `json:"photo"`
}

func ValidateFirstname(v *validator.Validator, firstname string) {
	v.Check(firstname != "", "firstname", "must be provided")
	v.Check(len(firstname) >= 3, "firstname", "must be at least 3 letters long")
	v.Check(len(firstname) <= 12, "firstname", "must not be more than 12 letters long")
}

func ValidateLastname(v *validator.Validator, lastname string) {
	v.Check(lastname != "", "lasstname", "must be provided")
	v.Check(len(lastname) >= 3, "lastname", "must be at least 3 letters long")
	v.Check(len(lastname) <= 12, "lasstname", "must not be more than 12 letters long")
}

func ValidateLichessUserName(v *validator.Validator, lichess_username string) {
	v.Check(lichess_username != "", "lichess_username ", "must be provided")
	v.Check(len(lichess_username) >= 3, "lichess_username ", "must be at least 3 letters long")
	v.Check(len(lichess_username) <= 12, "lichess_username ", "must not be more than 12 letters long")
}

func ValidatePhoneNumber(v *validator.Validator, phone_number string) {
	v.Check(phone_number != "", "phone_number ", "must be provided")
	v.Check(len(phone_number) >= 10, "phone_number ", "must be at least 10 letters long")
	v.Check(len(phone_number) <= 13, "phone_number ", "must not be more than 13 letters long")

}

func ValidateAccount(v *validator.Validator, account *Account) {
	ValidateFirstname(v, account.Firstname)
	ValidateLastname(v, account.Lastname)
	ValidateLichessUserName(v, account.LichessUsername)
	ValidatePhoneNumber(v, account.PhoneNumber)
}

type AccountModel struct {
	DB *sql.DB
}

var photoNull sql.NullByte
var chesscomNull sql.NullString

func (a AccountModel) Insert(account *Account) error {

	query := fmt.Sprintf(`
	INSERT INTO accounts (account_id,user_id, firstname,lastname,lichess_username, chesscom_username, phone_number)
	VALUES (%s, $1, $2, $3 , $4, $5, $6)
	RETURNING uuid, created_at, version`, "uuid_generate_v4()")

	args := []interface{}{account.UserID, account.Firstname, account.Lastname, account.LichessUsername, account.ChesscomUsername, account.PhoneNumber}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := a.DB.QueryRowContext(ctx, query, args...).Scan(&account.AccountID, &account.CreatedAt)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "accounts_lichess_username_key"`:
			return ErrDuplicateLichessUsername

		case err.Error() == `pq: duplicate key value violates unique constraint "accounts_phone_number_key"`:
			return ErrDuplicatePhonenumber

		case err.Error() == `pq: duplicate key value violates unique constraint "accounts_chesscom_username_key"`:
			return ErrDuplicateChesscomUsername
		default:
			return err
		}
	}
	return nil

}

func (a AccountModel) Get(user_id string) (*Account, error) {
	query := `
SELECT account_id,user_id, firstname,lastname,lichess_username, chesscom_username, phone_number, photo, created_at
FROM accounts
WHERE user_id = $1`
	var account Account
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := a.DB.QueryRowContext(ctx, query, user_id).Scan(
		&account.AccountID,
		&account.UserID,
		&account.Firstname,
		&account.Lastname,
		&account.LichessUsername,
		&chesscomNull,
		&account.PhoneNumber,
		&photoNull,
		&account.CreatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	var photo byte
	chesscom := ""

	if photoNull.Valid {
		photo = photoNull.Byte
	}

	if chesscomNull.Valid {
		chesscom = chesscomNull.String
	}

	account.Photo = photo
	account.ChesscomUsername = chesscom

	return &account, nil
}
