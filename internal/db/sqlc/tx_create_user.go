package db

import (
	"context"
	"fmt"
)

type CreateUserTxParams struct {
	CreateUserParams
	AfterCreate func(user User) error
}

type CreateUserTxResult struct {
	User User
}

func (store *SQLStore) CreateUserTx(ctx context.Context, arg CreateUserTxParams) (CreateUserTxResult, error) {
	fmt.Println("CreateUserTx")
	var result CreateUserTxResult

	err := store.ExecTx(ctx, func(q *Queries) error {
		var err error

		result.User, err = q.CreateUser(ctx, arg.CreateUserParams)
		if err != nil {
			return err
		}
		fmt.Println(result.User)
		fmt.Println("|", arg.AfterCreate, "|")
		return arg.AfterCreate(result.User)
	})
	fmt.Println("CreateUserTx2")
	return result, err
}
