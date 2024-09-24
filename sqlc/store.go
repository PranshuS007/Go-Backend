package db

import (
	"context"
	"database/sql"
	"fmt"
)

// Queries here is imported. This is actually like inheritence , Here we are extending Queries to be used in Struct
// Now Store provides all functions to execute db queries and transaction

/*
By embedding *Queries, you can call its methods directly on a Store instance. This simplifies the code by removing the need to reference the embedded struct explicitly. For example, if Queries has a method CreateAccount, you can do:

go
Copy code
store.CreateAccount(ctx, params)
Instead of:

go
Copy code
store.Queries.CreateAccount(ctx, params)
*/
type Store struct {
	*Queries
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{
		db:      db,
		Queries: New(db),
	}
}

/*
&Store : creates instance of Store
*Store : Returns the value of Store
*/

/*
WILL generate DB transaction. create a new Query object with transaction and call the callback function
with created query and commit or rollback the transaction based on the error
*/

/*
Value Receiver (func (store Store)): Creates a copy of the Store instance. Suitable for methods that do not modify the struct or for small structs.
Pointer Receiver (func (store *Store)): Works with the original Store instance. Suitable for methods that modify the struct or for large structs where copying would be inefficient.
*/
func (store *Store) execTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	q := New(tx)
	err = fn(q)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
		}
		return err
	}

	return tx.Commit()
}

type TransferTxParams struct {
	FromAccountID int64 `json:"from_account_id"`
	ToAccountID   int64 `json:"to_account_id"`
	Amount        int64 `json:"amount"`
}

type TransferTxResult struct {
	Transfer    Transfer `json:"transfer"`
	FromAccount Account  `json:"from_account"`
	ToAccount   Account  `json:"to_account"`
	FromEntry   Entry    `json:"from_entry"`
	ToEntry     Entry    `json:"to_entry"`
}

var txtKey = struct{}{}

// TransferTx performs a money transfer from one account to the another
// it creates new transfer record, add account entries, and update balance with single Transaction
func (store *Store) TransferTx(ctx context.Context, arg TransferTxParams) (TransferTxResult, error) {
	var result TransferTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		txName := ctx.Value(txtKey)

		fmt.Println(txName, "create Transfer")

		result.Transfer, err = q.CreateTransfer(ctx, CreateTransferParams{
			FromAccountID: arg.FromAccountID,
			ToAccountID:   arg.ToAccountID,
			Amount:        arg.Amount,
		})
		if err != nil {
			return err
		}

		fmt.Println(txName, "create entry 1")
		result.FromEntry, err = q.CreateEntry(ctx, CreateEntryParams{
			AccountID: arg.FromAccountID,
			Amount:    -arg.Amount,
		})
		if err != nil {
			return err
		}

		fmt.Println(txName, "create entry 2")
		result.ToEntry, err = q.CreateEntry(ctx, CreateEntryParams{
			AccountID: arg.ToAccountID,
			Amount:    arg.Amount,
		})
		if err != nil {
			return err
		}

		fmt.Println(txName, "get account 1")
		account1, err := q.GetAccountForUpdate(ctx, arg.FromAccountID)
		if err != nil {
			return nil
		}

		fmt.Println(txName, "update Account 1")
		result.FromAccount, err = q.UpdateAccount(ctx, UpdateAccountParams{
			ID:      arg.FromAccountID,
			Balance: account1.Balance - arg.Amount,
		})
		if err != nil {
			return err
		}

		fmt.Println(txName, "get Account 2")
		account2, err := q.GetAccountForUpdate(ctx, arg.ToAccountID)
		if err != nil {
			return nil
		}

		fmt.Println(txName, "update Account 2")
		result.ToAccount, err = q.UpdateAccount(ctx, UpdateAccountParams{
			ID:      arg.ToAccountID,
			Balance: account2.Balance + arg.Amount,
		})
		if err != nil {
			return err
		}
		return nil
	})
	return result, err
}
